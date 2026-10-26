#Requires -Version 5.1
<#
.SYNOPSIS
Exercises real OpenSSH bootstrap, SSH authentication, and SFTP on a disposable CI runner.
.DESCRIPTION
Never run on an operator workstation or an existing game server. This test changes
the runner's sshd service, SSH configuration, and firewall rules. The runner is
discarded after the job. Only Windows capability discovery/installation is mocked.
#>
[CmdletBinding()]
param([switch]$DisposableRunner)

$ErrorActionPreference = 'Stop'
if (!$DisposableRunner -or $env:GITHUB_ACTIONS -cne 'true' -or $env:RUNNER_ENVIRONMENT -cne 'github-hosted' -or !$env:RUNNER_TEMP) {
    throw 'This integration test requires -DisposableRunner on an ephemeral GitHub-hosted Windows runner.'
}

$repositoryRoot = Split-Path -Parent $PSScriptRoot
$artifactRoot = Join-Path $repositoryRoot '.tools\bootstrap-integration'
New-Item -ItemType Directory -Force -Path $artifactRoot | Out-Null
$taskRoot = Join-Path $env:RUNNER_TEMP ('dzdeploy-ssh-integration-' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $taskRoot | Out-Null
$script:processSequence = 0
$script:capabilityInstallAttempts = 0
$script:capabilityQueries = 0
$utf8 = New-Object Text.UTF8Encoding($false)
$transcriptStarted = $false

function Assert-Integration([bool]$Condition, [string]$Message) {
    if (!$Condition) { throw $Message }
}

function ConvertTo-IntegrationNativeArgument([string]$Value) {
    # ProcessStartInfo.Arguments is one Windows command line, including on PS 5.1.
    # Quote every argument using the Windows C runtime escaping rules.
    $escaped = [regex]::Replace($Value, '(\\*)"', '$1$1\"')
    $escaped = [regex]::Replace($escaped, '(\\+)$', '$1$1')
    return '"' + $escaped + '"'
}

function Invoke-IntegrationNative {
    param([string]$Executable, [string[]]$Arguments, [int]$TimeoutSeconds = 90)
    $script:processSequence++
    $stem = '{0:D2}-{1}' -f $script:processSequence, [IO.Path]::GetFileNameWithoutExtension($Executable)
    $process = New-Object Diagnostics.Process
    $process.StartInfo.FileName = $Executable
    $process.StartInfo.Arguments = ($Arguments | ForEach-Object { ConvertTo-IntegrationNativeArgument $_ }) -join ' '
    $process.StartInfo.UseShellExecute = $false
    $process.StartInfo.CreateNoWindow = $true
    $process.StartInfo.RedirectStandardOutput = $true
    $process.StartInfo.RedirectStandardError = $true
    try {
        if (!$process.Start()) { throw "Could not start $Executable" }
        $stdoutTask = $process.StandardOutput.ReadToEndAsync()
        $stderrTask = $process.StandardError.ReadToEndAsync()
        if (!$process.WaitForExit($TimeoutSeconds * 1000)) {
            $process.Kill()
            $process.WaitForExit()
            throw "$Executable timed out after $TimeoutSeconds seconds."
        }
        $stdout = $stdoutTask.GetAwaiter().GetResult()
        $stderr = $stderrTask.GetAwaiter().GetResult()
        [IO.File]::WriteAllText((Join-Path $artifactRoot "$stem.stdout.log"), $stdout, $utf8)
        [IO.File]::WriteAllText((Join-Path $artifactRoot "$stem.stderr.log"), $stderr, $utf8)
        if ($process.ExitCode -ne 0) { throw "$Executable exited $($process.ExitCode): $stderr`n$stdout" }
        return $stdout
    } finally { $process.Dispose() }
}

function Get-IntegrationFingerprint([string]$Keygen, [string]$PublicKeyPath) {
    $output = Invoke-IntegrationNative $Keygen @('-lf', $PublicKeyPath, '-E', 'sha256')
    if ($output -notmatch 'SHA256:[A-Za-z0-9+/]+') { throw 'Could not parse the real SSH host fingerprint.' }
    return $Matches[0]
}

try {
    Start-Transcript -Path (Join-Path $artifactRoot 'integration-transcript.log') -Force | Out-Null
    $transcriptStarted = $true
    . (Join-Path $PSScriptRoot 'bootstrap-windows.ps1')
    Assert-DeployNativePowerShell
    Assert-DeployAdministrator

    # Use the runner's existing client solely to generate an ephemeral test key.
    # The bootstrap's Windows capability source is deliberately unavailable below.
    $keygenCommand = Get-Command ssh-keygen.exe -CommandType Application -ErrorAction Stop
    $identity = Join-Path $taskRoot 'client_ed25519'
    Invoke-IntegrationNative $keygenCommand.Source @('-t', 'ed25519', '-N', '', '-f', $identity, '-C', 'dzdeploy-disposable-integration') | Out-Null
    $publicKey = [IO.File]::ReadAllText($identity + '.pub').Trim()
    $account = $env:USERNAME
    $port = 22222
    $fallbackDirectory = Join-Path $env:ProgramFiles 'DayZDeploy\OpenSSH\10.4.0.0p1-Preview'

    # Force the reported silent NotPresent failure without removing Windows
    # components or changing any Windows Update policy on the CI runner.
    $script:realPayloadTest = ${function:Test-DeploySshPayload}
    function Test-DeploySshPayload {
        param([string]$Directory)
        if (![string]::Equals([IO.Path]::GetFullPath($Directory), $fallbackDirectory, [StringComparison]::OrdinalIgnoreCase)) { return $false }
        return (& $script:realPayloadTest -Directory $Directory)
    }
    function Get-WindowsCapability {
        [CmdletBinding()]param([switch]$Online, [string]$Name)
        $script:capabilityQueries++
        return [pscustomobject]@{ Name = $Name; State = 'NotPresent' }
    }
    function Add-WindowsCapability {
        [CmdletBinding()]param([switch]$Online, [string]$Name)
        $script:capabilityInstallAttempts++
        return [pscustomobject]@{ RestartNeeded = $false }
    }

    Invoke-DayZSSHBootstrap -Key $publicKey -Sources '127.0.0.1/32' -Account $account -Port $port
    Assert-Integration ($script:capabilityInstallAttempts -gt 0) 'The test did not exercise the failed Windows capability source.'
    $service = Get-DeploySshService
    $expectedExecutable = Join-Path $fallbackDirectory 'sshd.exe'
    Assert-Integration ($null -ne $service -and $service.State -eq 'Running') 'The real sshd service is not running.'
    Assert-Integration ($service.PathName -match ('^\s*"?' + [regex]::Escape($expectedExecutable) + '(?:"|\s|$)')) 'sshd is not running the verified fallback executable.'

    $rule = Get-NetFirewallRule -Name 'DayZDeploy-SSH' -ErrorAction Stop
    $addresses = @($rule | Get-NetFirewallAddressFilter | Select-Object -ExpandProperty RemoteAddress)
    Assert-Integration ($addresses.Count -eq 1 -and $addresses[0] -in @('127.0.0.1', '127.0.0.1/32')) 'The SSH firewall rule is not restricted to the test source.'
    $rulePort = $rule | Get-NetFirewallPortFilter
    Assert-Integration ([string]$rulePort.LocalPort -eq [string]$port) 'The SSH firewall rule has the wrong port.'
    $ruleProgram = $rule | Get-NetFirewallApplicationFilter
    Assert-Integration ([string]::Equals($ruleProgram.Program, $expectedExecutable, [StringComparison]::OrdinalIgnoreCase)) 'The SSH firewall rule has the wrong executable.'

    $ssh = Join-Path $fallbackDirectory 'ssh.exe'
    $keygen = Join-Path $fallbackDirectory 'ssh-keygen.exe'
    $sftp = Join-Path $fallbackDirectory 'sftp.exe'
    $hostPublicKeyPath = Join-Path $env:ProgramData 'ssh\ssh_host_ed25519_key.pub'
    $firstFingerprint = Get-IntegrationFingerprint $keygen $hostPublicKeyPath
    $hostPublicKeyFields = [IO.File]::ReadAllText($hostPublicKeyPath).Trim() -split '\s+'
    $knownHosts = Join-Path $taskRoot 'known_hosts'
    # Trust the locally read host key, never an unauthenticated keyscan result.
    [IO.File]::WriteAllText($knownHosts, "[127.0.0.1]:$port $($hostPublicKeyFields[0]) $($hostPublicKeyFields[1])`n", $utf8)
    $options = @('-oBatchMode=yes', '-oIdentitiesOnly=yes', '-oPasswordAuthentication=no', '-oStrictHostKeyChecking=yes', '-oHostKeyAlgorithms=ssh-ed25519', "-oUserKnownHostsFile=$knownHosts", '-oGlobalKnownHostsFile=NUL', '-oConnectTimeout=15', '-i', $identity)
    $endpoint = "$account@127.0.0.1"
    $marker = 'DZDEPLOY_SSH_INTEGRATION_OK'
    $sshOutput = Invoke-IntegrationNative $ssh ($options + @('-p', [string]$port, $endpoint, "echo $marker"))
    Assert-Integration ($sshOutput.Contains($marker)) 'Public-key SSH command execution failed.'

    $upload = Join-Path $taskRoot 'upload.txt'
    $download = Join-Path $taskRoot 'download.txt'
    $remote = Join-Path $taskRoot 'remote.txt'
    $content = 'DayZDeploy real SFTP integration ' + [Guid]::NewGuid().ToString('N')
    [IO.File]::WriteAllText($upload, $content, $utf8)
    $uploadSftp = $upload.Replace('\', '/')
    $downloadSftp = $download.Replace('\', '/')
    $remoteSftp = '/' + $remote.Replace('\', '/')
    $batch = Join-Path $taskRoot 'sftp-batch.txt'
    [IO.File]::WriteAllText($batch, "put `"$uploadSftp`" `"$remoteSftp`"`nget `"$remoteSftp`" `"$downloadSftp`"`nbye`n", $utf8)
    Invoke-IntegrationNative $sftp ($options + @('-P', [string]$port, '-b', $batch, $endpoint)) | Out-Null
    Assert-Integration ([IO.File]::ReadAllText($download) -ceq $content) 'SFTP upload/download changed the file contents.'

    $installAttempts = $script:capabilityInstallAttempts
    $queries = $script:capabilityQueries
    Invoke-DayZSSHBootstrap -Key $publicKey -Sources '127.0.0.1/32' -Account $account -Port $port
    Assert-Integration ($script:capabilityInstallAttempts -eq $installAttempts -and $script:capabilityQueries -eq $queries) 'The second bootstrap did not reuse its healthy installation.'
    $secondFingerprint = Get-IntegrationFingerprint $keygen $hostPublicKeyPath
    Assert-Integration ($secondFingerprint -ceq $firstFingerprint) 'A repeated bootstrap replaced the SSH host key.'
    $sshOutput = Invoke-IntegrationNative $ssh ($options + @('-p', [string]$port, $endpoint, "echo $marker"))
    Assert-Integration ($sshOutput.Contains($marker)) 'SSH authentication failed after the second bootstrap.'
    Write-Host "Real Windows bootstrap integration passed: SSH, SFTP, rerun, and host fingerprint $firstFingerprint."
} finally {
    # Collect public diagnostics only. Private client and host keys are never artifacts.
    try {
        Get-CimInstance Win32_Service -Filter "Name='sshd'" | Select-Object Name, PathName, State, StartMode, StartName | Format-List | Out-File (Join-Path $artifactRoot 'sshd-service.log')
        Get-NetFirewallRule -Name 'DayZDeploy-SSH' -ErrorAction SilentlyContinue | Format-List | Out-File (Join-Path $artifactRoot 'sshd-firewall.log')
        $serverLog = Join-Path $env:ProgramData 'ssh\logs\sshd.log'
        if (Test-Path -LiteralPath $serverLog) { Copy-Item -LiteralPath $serverLog -Destination (Join-Path $artifactRoot 'sshd.log') }
    } catch { Write-Warning "Could not collect all integration diagnostics: $_" }
    Stop-Service sshd -ErrorAction SilentlyContinue
    if ($transcriptStarted) { Stop-Transcript | Out-Null }
}
