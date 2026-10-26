#Requires -Version 5.1
# Self-contained mocks: never install capabilities, modify a host firewall,
# change service settings, or require a third-party test module.
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'bootstrap-windows.ps1')

function Assert-Equal($Actual, $Expected, [string]$Message) {
    if ($Actual -cne $Expected) { throw "$Message`nExpected: $Expected`nActual: $Actual" }
}
function Assert-Throws([scriptblock]$Action, [string]$Message) {
    $failed = $false
    try { & $Action } catch { $failed = $true }
    if (!$failed) { throw $Message }
}
function Assert-ThrowsContaining([scriptblock]$Action, [string[]]$Fragments) {
    $failure = $null
    try { & $Action } catch { $failure = $_.Exception.Message }
    if (!$failure) { throw 'Expected operation to fail.' }
    foreach ($fragment in $Fragments) {
        if (!$failure.Contains($fragment)) { throw "Failure omitted '$fragment': $failure" }
    }
}

$taskRoot = Join-Path ([IO.Path]::GetTempPath()) ('dzdeploy-ssh-test-' + [Guid]::NewGuid().ToString('N'))
$savedWindows = $env:WINDIR
$savedData = $env:ProgramData
$savedProgramFiles = $env:ProgramFiles
$script:calls = @{}
$script:aclCapture = $null
$realZipInstaller = ${function:Install-DeployOpenSSHZip}
$realSetSshService = ${function:Set-DeploySshServiceInstallation}
$realRestoreSshService = ${function:Restore-DeploySshService}
function Count-Call([string]$Name) { if (!$script:calls.ContainsKey($Name)) { $script:calls[$Name] = 0 }; $script:calls[$Name]++ }
function Assert-DeployAdministrator { Count-Call 'admin' }
function Get-LocalUser { param($Name); Count-Call 'account'; return [pscustomobject]@{ Enabled = $script:userEnabled; SID = [pscustomobject]@{ Value = 'S-1-5-21-1-2-3-500' } } }
function Get-LocalGroupMember { param($SID); return [pscustomobject]@{ SID = [pscustomobject]@{ Value = 'S-1-5-21-1-2-3-500' } } }
function Write-MockCapabilityPayload([string]$Name, [string]$Directory = $sshDir) {
    $files = @('ssh-keygen.exe')
    if ($Name -eq $script:serverCapability) { $files = @('sshd.exe', 'sftp-server.exe', 'sshd_config_default') }
    foreach ($file in $files) {
        $content = 'mock executable; never execute'
        if ($file -eq 'sshd_config_default') { $content = $stock }
        [IO.File]::WriteAllText((Join-Path $Directory $file), $content, $utf8)
    }
}
function Get-WindowsCapability {
    [CmdletBinding()]param([switch]$Online, $Name)
    Count-Call 'capability'; Count-Call "query-$Name"
    if ($Name -eq $script:failQuery) { throw 'mock capability query failure 0x800f081f' }
    return [pscustomobject]@{ State = $script:capabilities[$Name] }
}
function Add-WindowsCapability {
    [CmdletBinding()]param([switch]$Online, $Name)
    Count-Call 'install'; Count-Call "install-$Name"
    if ($Name -eq $script:failInstall) { throw 'mock Windows Update failure 0x800f0954' }
    if ($Name -eq $script:serverCapability -and $script:capabilityCreatesService) {
        $script:serviceRecord = [pscustomobject]@{PathName='"' + (Join-Path $sshDir 'sshd.exe') + '"';StartMode='Auto';State='Running';StartName='LocalSystem'}
        $script:broadRule = [pscustomobject]@{Name='OpenSSH-Server-In-TCP';Enabled='True'}
        Count-Call 'capability-service-create'
    }
    $script:capabilities[$Name] = $script:postInstallState
    if ($script:postInstallState -eq 'Installed' -and !$script:skipPayload) { Write-MockCapabilityPayload $Name }
    return [pscustomobject]@{ RestartNeeded = $script:restartNeeded }
}
function Reset-MockOpenSSH([string]$Client = 'Installed', [string]$Server = 'Installed') {
    $script:calls = @{}
    $script:capabilities = @{ $script:clientCapability = $Client; $script:serverCapability = $Server }
    $script:postInstallState = 'Installed'
    $script:restartNeeded = $false
    $script:failQuery = ''
    $script:failInstall = ''
    $script:skipPayload = $false
    $script:serviceRecord = $null
    $script:mockZipFailure = ''
    $script:failServiceInstall = $false
    $script:mitigation = $false
    $script:capabilityCreatesService = $false
    $script:broadRule = [pscustomobject]@{Name='OpenSSH-Server-In-TCP';Enabled='True'}
    foreach ($file in @('ssh-keygen.exe', 'sshd.exe', 'sftp-server.exe', 'sshd_config_default')) {
        $path = Join-Path $sshDir $file
        if (Test-Path -LiteralPath $path) { Remove-Item -LiteralPath $path }
    }
    if ($Client -eq 'Installed') { Write-MockCapabilityPayload $script:clientCapability }
    if ($Server -eq 'Installed') { Write-MockCapabilityPayload $script:serverCapability }
}
function Get-DeploySshService {
    Count-Call 'service-query'
    if ($script:serviceRecord) { return $script:serviceRecord.PSObject.Copy() }
    return $null
}
function Install-DeployOpenSSHZip {
    Count-Call 'zip-fallback'
    if ($script:mockZipFailure) { throw $script:mockZipFailure }
    Write-MockCapabilityPayload $script:clientCapability $zipDir
    Write-MockCapabilityPayload $script:serverCapability $zipDir
    return [pscustomobject]@{Directory=$zipDir;Provider='MicrosoftZip';Version='10.4.0.0p1-Preview';ServiceName='sshd'}
}
function Set-DeploySshServiceInstallation {
    param($Installation, $PreviousService)
    Count-Call 'service-install'
    if ($script:failServiceInstall) { throw 'mock service registration failure' }
    $script:serviceRecord = [pscustomobject]@{PathName='"' + (Join-Path $Installation.Directory 'sshd.exe') + '"';StartMode='Manual';State='Stopped';StartName='LocalSystem'}
}
function Restore-DeploySshService {
    param($PreviousService)
    Count-Call 'service-restore'
    $script:serviceRecord = $PreviousService
    $script:lastStartup = 'Manual'
    if ($PreviousService) { $script:lastStartup = $PreviousService.StartMode; if ($script:lastStartup -eq 'Auto') { $script:lastStartup = 'Automatic' } }
}
function Set-DeploySshMitigation {
    Count-Call 'mitigation'
    $before = [pscustomobject]@{Exists=$script:mitigation;Value=$null;KeyExisted=$script:mitigation}
    $script:mitigation = $true
    return $before
}
function Restore-DeploySshMitigation { param($Snapshot); if ($Snapshot) { Count-Call 'mitigation-restore'; $script:mitigation = $Snapshot.Exists } }
function Assert-NoSSHMutation {
    foreach ($call in @('native', 'service-install', 'set-service', 'restart', 'start', 'firewall', 'acl', 'mitigation')) {
        Assert-Equal $script:calls.ContainsKey($call) $false "Prerequisite failure reached $call"
    }
}
function Set-Acl { param($LiteralPath, $AclObject); $script:aclCapture = $AclObject; Count-Call 'acl' }
function Get-Acl { param($LiteralPath); return (New-Object Security.AccessControl.FileSecurity) }
function Invoke-DeployNative {
    param($Executable, [string[]]$ArgList)
    Count-Call 'native'
    if (!(Test-Path -LiteralPath $Executable -PathType Leaf)) { throw "Native executable is missing: $Executable" }
    if ($ArgList[0] -eq '-A') {
        foreach ($suffix in @('', '.pub')) {
            $hostPath = Join-Path $stateDir "ssh_host_ed25519_key$suffix"
            if (!(Test-Path -LiteralPath $hostPath)) { [IO.File]::WriteAllText($hostPath, 'mock-host-key' + $suffix, $utf8) }
        }
    }
    if ($ArgList[0] -eq 'privs') { $script:lastPrivileges = $ArgList[2] }
    if ($ArgList[0] -eq 'delete') { $script:serviceRecord = $null }
    if ($ArgList[0] -eq '-t' -and $script:failValidate) { throw 'mock sshd configuration failure' }
    if ($ArgList[0] -eq '-lf') { return '256 SHA256:trusted-test-key host (ED25519)' }
}
function Get-Service {
    param($Name)
    Count-Call 'service-query'
    $s = [pscustomobject]@{ Status = 'Running'; StartType = 'Manual' }
    $s | Add-Member -MemberType ScriptMethod -Name WaitForStatus -Value { param($State, $Timeout) }
    return $s
}
function New-Service {
    [CmdletBinding()]param($Name, $DisplayName, $BinaryPathName, $StartupType)
    Count-Call 'new-service'
    $script:serviceRecord = [pscustomobject]@{PathName=$BinaryPathName;StartMode=$StartupType;State='Stopped';StartName='LocalSystem'}
}
function Invoke-CimMethod {
    [CmdletBinding()]param($InputObject, $MethodName, $Arguments)
    Count-Call 'service-change'
    if ($script:failCimChange) { return [pscustomobject]@{ReturnValue=5} }
    $script:serviceRecord.PathName = $Arguments.PathName
    return [pscustomobject]@{ReturnValue=0}
}
function Set-Service { param($Name, $StartupType); $script:lastStartup = $StartupType; if ($script:serviceRecord) { $script:serviceRecord.StartMode = $StartupType }; Count-Call 'set-service' }
function Restart-Service { param($Name); Count-Call 'restart'; if ($script:failRestart) { $script:failRestart = $false; throw 'mock startup failure' }; if ($script:serviceRecord) { $script:serviceRecord.State='Running' } }
function Start-Service { param($Name); Count-Call 'start'; if ($script:failRestart) { $script:failRestart=$false; throw 'mock startup failure' }; if ($script:serviceRecord) { $script:serviceRecord.State='Running' } }
function Stop-Service { [CmdletBinding()]param($Name); Count-Call 'stop'; if ($script:serviceRecord) { $script:serviceRecord.State='Stopped' } }
function Get-NetFirewallRule {
    [CmdletBinding()]param($Name)
    if ($Name -eq 'OpenSSH-Server-In-TCP') { if ($script:broadRule) { return $script:broadRule.PSObject.Copy() }; return $null }
    if ($Name -eq 'sshd') { return $null }
    return $script:managedRule
}
function Get-NetFirewallPortFilter { param([Parameter(ValueFromPipeline)]$InputObject); process { return [pscustomobject]@{Protocol=$InputObject.Protocol;LocalPort=$InputObject.LocalPort} } }
function Get-NetFirewallAddressFilter { param([Parameter(ValueFromPipeline)]$InputObject); process { return [pscustomobject]@{RemoteAddress=$InputObject.RemoteAddress} } }
function Get-NetFirewallApplicationFilter { param([Parameter(ValueFromPipeline)]$InputObject); process { return [pscustomobject]@{Program=$InputObject.Program} } }
function New-NetFirewallRule {
    param($Name, $DisplayName, $Direction, $Action, $Protocol, $LocalPort, $RemoteAddress, $Program, $Profile, $Enabled)
    $script:lastRule = @{} + $PSBoundParameters
    $ruleEnabled = $Enabled
    if (!$ruleEnabled) { $ruleEnabled = 'True' }
    $newRule = [pscustomobject]@{Name=$Name;DisplayName=$DisplayName;Direction=$Direction;Action=$Action;Protocol=$Protocol;LocalPort=$LocalPort;RemoteAddress=$RemoteAddress;Program=$Program;Profile=$Profile;Enabled=$ruleEnabled}
    if ($Name -eq 'OpenSSH-Server-In-TCP') { $script:broadRule=$newRule }
    else { $script:managedRule=$newRule }
    Count-Call 'firewall'
}
function Disable-NetFirewallRule {
    [CmdletBinding()]param($Name, [Parameter(ValueFromPipeline)]$InputObject)
    process {
        Count-Call 'disable-broad'
        if (!$Name -and $InputObject) { $Name=$InputObject.Name }
        if ($Name -eq 'OpenSSH-Server-In-TCP' -and $script:broadRule) { $script:broadRule.Enabled='False' }
    }
}
function Set-NetFirewallRule { param($Name, $Enabled); Count-Call 'restore-broad'; if ($Name -eq 'OpenSSH-Server-In-TCP' -and $script:broadRule) { $script:broadRule.Enabled=$Enabled } }
function Remove-NetFirewallRule {
    [CmdletBinding()]param($Name, [Parameter(ValueFromPipeline)]$InputObject)
    process {
        Count-Call 'remove-rule'
        if (!$Name -and $InputObject) { $Name=$InputObject.Name }
        if ($Name -eq 'OpenSSH-Server-In-TCP') { $script:broadRule=$null; Count-Call 'remove-broad' }
        else { $script:managedRule=$null }
    }
}
function New-TestZipArchive([string]$Path, [hashtable]$Files) {
    Add-Type -AssemblyName System.IO.Compression
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $archive = [IO.Compression.ZipFile]::Open($Path, [IO.Compression.ZipArchiveMode]::Create)
    try {
        foreach ($name in $Files.Keys) {
            $writer = New-Object IO.StreamWriter($archive.CreateEntry($name).Open())
            try { $writer.Write($Files[$name]) } finally { $writer.Dispose() }
        }
    } finally { $archive.Dispose() }
}

try {
    New-Item -ItemType Directory -Path $taskRoot | Out-Null
    $env:WINDIR = Join-Path $taskRoot 'Windows'
    $env:ProgramData = Join-Path $taskRoot 'ProgramData'
    $env:ProgramFiles = Join-Path $taskRoot 'ProgramFiles'
    $sshDir = Join-Path $env:WINDIR 'System32\OpenSSH'
    $zipDir = Join-Path $taskRoot 'ProgramFiles\DayZDeploy\OpenSSH\10.4.0.0p1-Preview'
    $stateDir = Join-Path $env:ProgramData 'ssh'
    New-Item -ItemType Directory -Force -Path $sshDir, $stateDir, $zipDir | Out-Null
    $utf8 = New-Object Text.UTF8Encoding($false)
    $stock = "# stock`r`nPort 22`r`nSubsystem sftp sftp-server.exe`r`nMatch Group administrators`r`n    AuthorizedKeysFile __PROGRAMDATA__/ssh/administrators_authorized_keys`r`n"
    [IO.File]::WriteAllText((Join-Path $sshDir 'sshd_config_default'), $stock, $utf8)
    $blob = [byte[]]@(0, 0, 0, 11) + [Text.Encoding]::ASCII.GetBytes('ssh-ed25519') + [byte[]]@(0, 0, 0, 32) + (New-Object byte[] 32)
    $key = 'ssh-ed25519 ' + [Convert]::ToBase64String($blob)
    $pub = Join-Path $taskRoot 'id_ed25519.pub'
    [IO.File]::WriteAllText($pub, "$key test-client", $utf8)
    $script:userEnabled = $true
    $script:failValidate = $false
    $script:failRestart = $false
    $script:managedRule = $null
    $script:clientCapability = 'OpenSSH.Client~~~~0.0.1.0'
    $script:serverCapability = 'OpenSSH.Server~~~~0.0.1.0'
    Reset-MockOpenSSH

    # Exercise the actual service transition helpers with mocked Windows APIs.
    [IO.File]::WriteAllText((Join-Path $env:WINDIR 'System32\sc.exe'), 'mock sc executable', $utf8)
    $serviceInstallation = [pscustomobject]@{Directory=$zipDir;Provider='MicrosoftZip';Version=(Get-DeployOpenSSHZipSpec).Version;ServiceName='sshd'}
    & $realSetSshService -Installation $serviceInstallation -PreviousService $null
    Assert-Equal $script:serviceRecord.StartName 'LocalSystem' 'New service must use LocalSystem'
    Assert-Equal $script:serviceRecord.State 'Stopped' 'Service registration started SSH before scoped configuration'
    Assert-Equal $script:serviceRecord.PathName ('"' + (Join-Path $zipDir 'sshd.exe') + '"') 'New service binary was not quoted'
    Assert-Equal $script:lastPrivileges 'SeAssignPrimaryTokenPrivilege/SeTcbPrivilege/SeBackupPrivilege/SeRestorePrivilege/SeImpersonatePrivilege' 'New service privileges differ from upstream'
    & $realRestoreSshService -PreviousService $null
    Assert-Equal ($null -eq $script:serviceRecord) $true 'Rollback retained a newly created service'
    $originalService = [pscustomobject]@{PathName='"' + (Join-Path $sshDir 'sshd.exe') + '"';StartMode='Manual';State='Running';StartName='LocalSystem'}
    $script:serviceRecord = $originalService.PSObject.Copy()
    & $realSetSshService -Installation $serviceInstallation -PreviousService $originalService
    Assert-Equal $script:serviceRecord.PathName ('"' + (Join-Path $zipDir 'sshd.exe') + '"') 'Existing service was not repointed'
    Assert-Equal $script:serviceRecord.State 'Stopped' 'Service migration did not stop old executable'
    & $realRestoreSshService -PreviousService $originalService
    Assert-Equal $script:serviceRecord.PathName $originalService.PathName 'Migration rollback did not restore executable'
    Assert-Equal $script:serviceRecord.StartMode $originalService.StartMode 'Migration rollback changed startup type'
    Assert-Equal $script:serviceRecord.State 'Running' 'Migration rollback did not restore running state'
    $disabledRunningService = $originalService.PSObject.Copy()
    $disabledRunningService.StartMode = 'Disabled'
    $script:serviceRecord.PathName = '"' + (Join-Path $zipDir 'sshd.exe') + '"'
    $script:serviceRecord.State = 'Stopped'
    & $realRestoreSshService -PreviousService $disabledRunningService
    Assert-Equal $script:serviceRecord.StartMode 'Disabled' 'Rollback lost disabled startup on a previously running service'
    Assert-Equal $script:serviceRecord.State 'Running' 'Rollback did not temporarily enable startup to restore a running disabled service'
    $script:serviceRecord.StartName = 'custom-account'
    Assert-ThrowsContaining { & $realSetSshService -Installation $serviceInstallation -PreviousService $script:serviceRecord } @('LocalSystem')
    Assert-Equal $script:serviceRecord.PathName $originalService.PathName 'Incompatible service account was changed'
    $script:serviceRecord.StartName = 'LocalSystem'
    $script:failCimChange = $true
    Assert-ThrowsContaining { & $realSetSshService -Installation $serviceInstallation -PreviousService $originalService } @('failed', '5')
    $script:failCimChange = $false
    Reset-MockOpenSSH

    # Exercise trust boundaries without network access or executable execution.
    $actualSpec = Get-DeployOpenSSHZipSpec
    Assert-Equal $actualSpec.Version '10.4.0.0p1-Preview' 'Fallback version changed without updating expected fixture'
    Assert-Equal $actualSpec.SHA256 'b67ff8becd203502190ec42ac7787c24c2cd3325ed7700485fe7a1c83d2a7957' 'Fallback archive digest is not pinned'
    $untrusted = Join-Path $taskRoot 'unsigned.exe'
    [IO.File]::WriteAllText($untrusted, 'not a Microsoft executable', $utf8)
    Assert-Throws { Assert-DeployMicrosoftSignature -Path $untrusted } 'Unsigned executable accepted'
    Assert-ThrowsContaining { Assert-DeployOpenSSHArchiveHash -Path $untrusted -ExpectedSHA256 ('0' * 64) } @('SHA-256')
    Assert-ThrowsContaining { Get-DeployOpenSSHArchive -Uri 'http://example.invalid/openssh.zip' -Destination (Join-Path $taskRoot 'forbidden-download.zip') -ExpectedSHA256 ('0' * 64) } @('Unsafe OpenSSH download URL')
    Assert-Equal (Test-Path -LiteralPath (Join-Path $taskRoot 'forbidden-download.zip')) $false 'HTTP rejection wrote a download'

    foreach ($entryName in @('OpenSSH-Win64/../escaped.txt', 'OpenSSH-Win64/C:/escaped.txt', '/OpenSSH-Win64/escaped.txt', 'OtherRoot/escaped.txt')) {
        $badArchive = Join-Path $taskRoot ('unsafe-' + [Guid]::NewGuid().ToString('N') + '.zip')
        $destination = Join-Path $taskRoot ('extract-' + [Guid]::NewGuid().ToString('N'))
        New-TestZipArchive $badArchive @{ $entryName = 'must not escape' }
        Assert-Throws { Expand-DeployOpenSSHArchive -ArchivePath $badArchive -Destination $destination } "Unsafe archive entry accepted: $entryName"
        Assert-Equal (Test-Path -LiteralPath (Join-Path $taskRoot 'escaped.txt')) $false 'Archive traversal escaped extraction'
    }

    $fixtureFiles = @{}
    foreach ($file in @('ssh-keygen.exe', 'sshd.exe', 'sftp-server.exe', 'sshd_config_default', 'sshd-auth.exe', 'sshd-session.exe', 'libcrypto.dll')) { $fixtureFiles['OpenSSH-Win64/' + $file] = 'fixture ' + $file }
    $fixtureArchive = Join-Path $taskRoot 'valid-fixture.zip'
    New-TestZipArchive $fixtureArchive $fixtureFiles
    $script:fixtureSpec = [pscustomobject]@{Version=$actualSpec.Version;Url='https://example.invalid/fixture.zip';SHA256=(Get-FileHash -LiteralPath $fixtureArchive -Algorithm SHA256).Hash.ToLowerInvariant()}
    $savedSpec = ${function:Get-DeployOpenSSHZipSpec}
    $savedDownload = ${function:Get-DeployOpenSSHArchive}
    $savedSignature = ${function:Assert-DeployMicrosoftSignature}
    $savedExpand = ${function:Expand-DeployOpenSSHArchive}
    $runtimeProgramFiles = $env:ProgramFiles
    $env:ProgramFiles = Join-Path $taskRoot 'PackageTrustProgramFiles'
    $fixtureInstallPath = Join-Path $env:ProgramFiles ('DayZDeploy\OpenSSH\' + $actualSpec.Version)
    try {
        function Get-DeployOpenSSHZipSpec { return $script:fixtureSpec }
        function Get-DeployOpenSSHArchive {
            param($Uri, $Destination, $ExpectedSHA256)
            if ($script:truncateDownload) { [IO.File]::WriteAllText($Destination, 'truncated download', $utf8) }
            else { Copy-Item -LiteralPath $fixtureArchive -Destination $Destination }
        }
        function Assert-DeployMicrosoftSignature {
            param($Path)
            if ($script:rejectSignature) { throw 'fixture Microsoft signature rejected' }
        }
        $script:truncateDownload = $true
        Assert-ThrowsContaining { & $realZipInstaller } @('SHA-256')
        Assert-Equal (Test-Path -LiteralPath $fixtureInstallPath) $false 'Incomplete download was installed'
        $script:truncateDownload = $false
        $script:rejectSignature = $true
        Assert-ThrowsContaining { & $realZipInstaller } @('signature rejected')
        Assert-Equal (Test-Path -LiteralPath $fixtureInstallPath) $false 'Rejected signature was installed'
        $script:rejectSignature = $false
        function Expand-DeployOpenSSHArchive { param($ArchivePath, $Destination); New-Item -ItemType Directory -Path $Destination | Out-Null; throw 'fixture interrupted extraction' }
        Assert-ThrowsContaining { & $realZipInstaller } @('interrupted extraction')
        Assert-Equal (Test-Path -LiteralPath $fixtureInstallPath) $false 'Interrupted extraction was installed'
        Assert-Equal (@(Get-ChildItem -LiteralPath (Split-Path $fixtureInstallPath) -Filter '.stage-*').Count) 0 'Failed package staging was not cleaned'
        Set-Item -Path Function:\Expand-DeployOpenSSHArchive -Value $savedExpand
        $fixtureInstallation = & $realZipInstaller
        Assert-Equal $fixtureInstallation.Directory $fixtureInstallPath 'Verified fixture package was not installed'
        Assert-Equal (Test-DeployOpenSSHZipInstallation -Directory $fixtureInstallPath) $true 'Verified fixture manifest rejected'
        Assert-Equal (& $realZipInstaller).Directory $fixtureInstallPath 'Verified package could not be reused'
        $tamperedFile = Join-Path $fixtureInstallPath 'sshd.exe'
        [IO.File]::WriteAllText($tamperedFile, 'tampered executable', $utf8)
        Assert-Equal (Test-DeployOpenSSHZipInstallation -Directory $fixtureInstallPath) $false 'Tampered package manifest accepted'
        Assert-ThrowsContaining { & $realZipInstaller } @('incomplete or corrupt')
        Assert-Equal ([IO.File]::ReadAllText($tamperedFile)) 'tampered executable' 'Active corrupted package was overwritten'
    } finally {
        Set-Item -Path Function:\Get-DeployOpenSSHZipSpec -Value $savedSpec
        Set-Item -Path Function:\Get-DeployOpenSSHArchive -Value $savedDownload
        Set-Item -Path Function:\Assert-DeployMicrosoftSignature -Value $savedSignature
        Set-Item -Path Function:\Expand-DeployOpenSSHArchive -Value $savedExpand
        $env:ProgramFiles = $runtimeProgramFiles
    }
    Reset-MockOpenSSH

    Assert-Equal (Test-DeploySource '203.0.113.10/32') $true 'IPv4 CIDR'
    Assert-Equal (Test-DeploySource '2001:db8::1/128') $true 'IPv6 CIDR'
    foreach ($invalid in @('Any', '*', '0.0.0.0/0', '::/0', '1.2.3.4/33', 'bad/32', '1.2.3.4/32/2')) { Assert-Equal (Test-DeploySource $invalid) $false "Unsafe source $invalid" }
    Assert-Throws { ConvertTo-DeployPublicKey 'not a public key' } 'Invalid key accepted'
    Assert-Throws { ConvertTo-DeployPublicKey '' } 'Missing key accepted'
    Assert-Throws { ConvertTo-DeployPublicKey 'missing-key' } 'Unconfigured embedded key accepted'
    Assert-Throws { ConvertTo-DeployPublicKey "$key`n$key" } 'Multiple keys accepted'
    Assert-Throws { New-DeploySshConfig $stock 'Administrator#injection' 22 } 'Config injection accepted'
    $config = New-DeploySshConfig $stock 'Administrator' 2222
    Assert-Equal ([regex]::Matches($config, '(?m)^Port 2222\r?$').Count) 1 'Custom port missing'
    Assert-Equal ($config.IndexOf('Match User administrator') -lt $config.IndexOf('Match Group administrators')) $true 'Admin match precedence incorrect'
    Assert-Equal $config (New-DeploySshConfig $config 'Administrator' 2222) 'Config rerun changed output'
    $absoluteSftp = Join-Path $zipDir 'sftp-server.exe'
    $zipConfig = New-DeploySshConfig $stock 'Administrator' 2222 -SftpPath $absoluteSftp -FileLogging
    Assert-Equal ([regex]::Matches($zipConfig, '(?m)^Subsystem sftp ').Count) 1 'ZIP config duplicated SFTP subsystem'
    Assert-Equal ($zipConfig.Contains('"' + $absoluteSftp.Replace('\', '/') + '"')) $true 'ZIP config must use absolute quoted SFTP path'
    Assert-Equal ([regex]::Matches($zipConfig, '(?m)^SyslogFacility LOCAL0\r?$').Count) 1 'ZIP config must enable file logging'
    Assert-Equal (Merge-DeployAuthorizedKeys "# existing`n$key old-comment`n$key duplicate" $key) "# existing`r`n$key old-comment`r`n" 'Key deduplication/preservation'

    Set-DeployPrivateAcl $pub
    $rules = @($script:aclCapture.GetAccessRules($true, $false, [Security.Principal.SecurityIdentifier]))
    Assert-Equal (@($rules.IdentityReference.Value | Sort-Object) -join ',') 'S-1-5-18,S-1-5-32-544' 'Key ACL must only include SYSTEM and Administrators'
    Assert-Equal $script:aclCapture.AreAccessRulesProtected $true 'Key ACL inherits permissions'

    $script:calls = @{}
    Invoke-DayZSSHBootstrap -KeyPath $pub -Sources '203.0.113.10/32' -Port 2222 -WhatIf
    Assert-Equal $script:calls.ContainsKey('install') $false 'WhatIf installed OpenSSH'
    Assert-Equal $script:calls.ContainsKey('firewall') $false 'WhatIf changed firewall'
    Assert-Equal $script:calls.ContainsKey('native') $false 'WhatIf ran native commands'
    Assert-Equal $script:calls.ContainsKey('capability') $false 'WhatIf queried Windows capabilities'
    Invoke-DayZSSHBootstrap -Key "$key embedded-client" -Sources '203.0.113.10/32' -WhatIf
    Assert-Equal $script:calls.ContainsKey('native') $false 'Embedded-key WhatIf ran native commands'
    Assert-Throws { Invoke-DayZSSHBootstrap -Key $key -KeyPath $pub -Sources '203.0.113.10/32' } 'Ambiguous key inputs accepted'
    Assert-Throws { Invoke-DayZSSHBootstrap -Key 'missing-key' -Sources '203.0.113.10/32' } 'Unconfigured embedded key reached setup'
    Assert-Equal $script:calls.ContainsKey('capability') $false 'Invalid key changed Windows capabilities'
    Assert-Throws { Invoke-DayZSSHBootstrap -KeyPath $pub -Sources '0.0.0.0/0' } 'Unrestricted source accepted'
    $script:userEnabled = $false
    Assert-Throws { Invoke-DayZSSHBootstrap -KeyPath $pub -Sources '203.0.113.10/32' } 'Disabled account accepted'
    $script:userEnabled = $true

    Assert-DeployNativePowerShell -Is64BitOperatingSystem $true -Is64BitProcess $true
    Assert-ThrowsContaining { Assert-DeployNativePowerShell -Is64BitOperatingSystem $true -Is64BitProcess $false } @('64-bit Windows PowerShell', 'Sysnative', 'LocalAccounts')
    $nativeShellCheck = ${function:Assert-DeployNativePowerShell}
    try {
        function Assert-DeployNativePowerShell { & $nativeShellCheck -Is64BitOperatingSystem $true -Is64BitProcess $false }
        $script:calls = @{}
        Assert-ThrowsContaining { Invoke-DayZSSHBootstrap -Key $key -Sources '203.0.113.10/32' } @('64-bit Windows PowerShell')
        Assert-Equal $script:calls.ContainsKey('account') $false '32-bit shell reached LocalAccounts'
        Assert-Equal $script:calls.ContainsKey('capability') $false '32-bit shell reached capability installation'
        Assert-NoSSHMutation
    } finally { Set-Item -Path Function:\Assert-DeployNativePowerShell -Value $nativeShellCheck }

    # Complete existing installations are independent of capability status.
    Reset-MockOpenSSH -Client 'NotPresent' -Server 'NotPresent'
    $existingDirectory = Join-Path $taskRoot 'Existing OpenSSH'
    New-Item -ItemType Directory -Path $existingDirectory | Out-Null
    Write-MockCapabilityPayload $script:clientCapability $existingDirectory
    Write-MockCapabilityPayload $script:serverCapability $existingDirectory
    $script:serviceRecord = [pscustomobject]@{PathName='"' + (Join-Path $existingDirectory 'sshd.exe') + '"';StartMode='Manual';State='Running';StartName='LocalSystem'}
    $existing = Install-DeployOpenSSH
    Assert-Equal $existing.Directory $existingDirectory 'Existing service outside System32 was ignored'
    Assert-Equal $script:calls.ContainsKey('capability') $false 'Healthy service queried capabilities'
    Assert-Equal $script:calls.ContainsKey('zip-fallback') $false 'Healthy service downloaded fallback'

    # Regression: Server installed with Client/keygen absent.
    Reset-MockOpenSSH -Client 'NotPresent'
    Assert-Equal (Test-Path -LiteralPath (Join-Path $sshDir 'ssh-keygen.exe')) $false 'Missing-client setup still had keygen'
    Assert-Equal (Install-DeployOpenSSH).Directory $sshDir 'Missing client was not installed'
    Assert-Equal $script:calls["install-$script:clientCapability"] 1 'Client capability was not installed exactly once'
    Assert-Equal $script:calls.ContainsKey("install-$script:serverCapability") $false 'Existing server capability was reinstalled'
    Assert-Equal $script:calls["query-$script:clientCapability"] 2 'Client install state was not verified after installation'
    Assert-Equal (Test-Path -LiteralPath (Join-Path $sshDir 'ssh-keygen.exe')) $true 'Client installation did not supply keygen'

    Reset-MockOpenSSH -Client 'NotPresent' -Server 'NotPresent'
    Assert-Equal (Install-DeployOpenSSH).Directory $sshDir 'Fresh host prerequisites failed'
    Assert-Equal $script:calls.install 2 'Fresh host did not install both capabilities'
    $script:calls = @{}
    Assert-Equal (Install-DeployOpenSSH).Directory $sshDir 'Installed host prerequisites failed'
    Assert-Equal $script:calls.ContainsKey('install') $false 'Healthy capabilities were reinstalled'

    Reset-MockOpenSSH -Client 'NotPresent'
    $script:postInstallState = 'NotPresent'
    $fallback = Install-DeployOpenSSH
    Assert-Equal $fallback.Directory $zipDir 'Silent Windows install failure did not recover automatically'
    Assert-Equal $fallback.Provider 'MicrosoftZip' 'Silent failure selected wrong provider'
    Assert-Equal $script:calls['zip-fallback'] 1 'Silent failure did not invoke fallback exactly once'

    foreach ($file in @('ssh-keygen.exe', 'sshd.exe', 'sftp-server.exe', 'sshd_config_default')) {
        Reset-MockOpenSSH
        Remove-Item -LiteralPath (Join-Path $sshDir $file)
        Assert-Equal (Install-DeployOpenSSH).Directory $zipDir "Missing $file did not select complete fallback"
        Assert-Equal $script:calls['zip-fallback'] 1 "Missing $file did not invoke fallback"
    }
    Reset-MockOpenSSH -Client 'NotPresent'
    $script:skipPayload = $true
    Assert-Equal (Install-DeployOpenSSH).Directory $zipDir 'Reported install success without files did not recover'

    Reset-MockOpenSSH -Client 'NotPresent'
    $script:restartNeeded = $true
    Assert-ThrowsContaining { Invoke-DayZSSHBootstrap -Key $key -Sources '203.0.113.10/32' } @($script:clientCapability, 'requires a reboot')
    Assert-Equal $script:calls.ContainsKey('zip-fallback') $false 'Reboot requirement incorrectly invoked fallback'
    Assert-NoSSHMutation
    Reset-MockOpenSSH -Client 'InstallPending'
    Assert-ThrowsContaining { Invoke-DayZSSHBootstrap -Key $key -Sources '203.0.113.10/32' } @('InstallPending', 'Reboot')
    Assert-Equal $script:calls.ContainsKey('install') $false 'Pending capability was reinstalled'
    Assert-Equal $script:calls.ContainsKey('zip-fallback') $false 'Pending capability incorrectly invoked fallback'
    Assert-NoSSHMutation
    Reset-MockOpenSSH -Client 'NotPresent'
    $script:postInstallState = 'InstallPending'
    Assert-ThrowsContaining { Invoke-DayZSSHBootstrap -Key $key -Sources '203.0.113.10/32' } @('InstallPending', 'Reboot')
    Assert-Equal $script:calls.ContainsKey('zip-fallback') $false 'Postinstall pending incorrectly invoked fallback'
    Assert-NoSSHMutation

    Reset-MockOpenSSH -Server 'NotPresent'
    $script:failInstall = $script:serverCapability
    Assert-Equal (Install-DeployOpenSSH).Directory $zipDir 'Windows Update exception did not recover'
    Reset-MockOpenSSH
    $script:failQuery = $script:clientCapability
    Remove-Item -LiteralPath (Join-Path $sshDir 'ssh-keygen.exe')
    Assert-Equal (Install-DeployOpenSSH).Directory $zipDir 'Capability query exception did not recover'
    Reset-MockOpenSSH -Client 'NotPresent'
    $script:postInstallState = 'NotPresent'
    $script:mockZipFailure = 'mock download checksum mismatch'
    Assert-ThrowsContaining { Invoke-DayZSSHBootstrap -Key $key -Sources '203.0.113.10/32' } @('checksum mismatch')
    Assert-NoSSHMutation
    # A capability installer can create service/firewall state before returning
    # failure. Rollback must include those effects even before main setup begins.
    Reset-MockOpenSSH -Client 'NotPresent' -Server 'NotPresent'
    $script:postInstallState = 'NotPresent'
    $script:capabilityCreatesService = $true
    $script:broadRule = $null
    $script:mockZipFailure = 'mock fallback rejected after capability side effects'
    Assert-ThrowsContaining { Invoke-DayZSSHBootstrap -Key $key -Sources '203.0.113.10/32' } @('fallback rejected')
    Assert-Equal $script:calls['capability-service-create'] 1 'Capability side-effect fixture did not run'
    Assert-Equal ($null -eq $script:serviceRecord) $true 'Prerequisite failure retained a newly created sshd service'
    Assert-Equal ($null -eq $script:broadRule) $true 'Prerequisite failure retained a newly created broad firewall rule'
    Assert-Equal $script:calls['service-restore'] 1 'Prerequisite service side effects were not rolled back'
    Assert-Equal $script:calls.ContainsKey('native') $false 'Capability failure reached host key generation'

    Reset-MockOpenSSH -Client 'NotPresent' -Server 'NotPresent'
    $script:serviceRecord = [pscustomobject]@{PathName='"' + (Join-Path $sshDir 'sshd.exe') + '"';StartMode='Disabled';State='Stopped';StartName='LocalSystem'}
    $script:broadRule.Enabled = 'False'
    $script:postInstallState = 'NotPresent'
    $script:capabilityCreatesService = $true
    $script:mockZipFailure = 'mock fallback rejected after capability side effects'
    Assert-ThrowsContaining { Invoke-DayZSSHBootstrap -Key $key -Sources '203.0.113.10/32' } @('fallback rejected')
    Assert-Equal $script:serviceRecord.StartMode 'Disabled' 'Prerequisite failure changed existing service startup'
    Assert-Equal $script:serviceRecord.State 'Stopped' 'Prerequisite failure changed existing service running state'
    Assert-Equal $script:broadRule.Enabled 'False' 'Prerequisite failure enabled a previously disabled broad rule'
    Reset-MockOpenSSH

    # Exercise the script entry point using a personalized copy and no .pub file.
    # Override only host identity checks so -WhatIf can run without elevation.
    $singleScript = Join-Path $taskRoot 'bootstrap-windows.ps1'
    $template = [IO.File]::ReadAllText((Join-Path $PSScriptRoot 'bootstrap-windows.ps1'))
    $parseTokens = $null
    $parseErrors = $null
    $bootstrapAst = [Management.Automation.Language.Parser]::ParseInput($template, [ref]$parseTokens, [ref]$parseErrors)
    Assert-Equal $parseErrors.Count 0 'Bootstrap script has syntax errors'
    $publicKeyParameter = $bootstrapAst.ParamBlock.Parameters | Where-Object { $_.Name.VariablePath.UserPath -eq 'PublicKey' }
    $shippedKey = ConvertTo-DeployPublicKey $publicKeyParameter.DefaultValue.Value
    Assert-Equal ($shippedKey.StartsWith('ssh-ed25519 ')) $true 'Tracked script must embed the deployment public key'
    $identityMocks = @'
function Assert-DeployAdministrator {}
function Get-LocalUser { param($Name); return [pscustomobject]@{ Enabled = $true; SID = [pscustomobject]@{ Value = 'test-admin' } } }
function Get-LocalGroupMember { param($SID); return [pscustomobject]@{ SID = [pscustomobject]@{ Value = 'test-admin' } } }
'@
    $personalized = $template.Replace($publicKeyParameter.DefaultValue.Extent.Text, "'$key'").Replace('# Dot-sourcing exposes', $identityMocks + "`r`n# Dot-sourcing exposes")
    [IO.File]::WriteAllText($singleScript, $personalized, $utf8)
    Remove-Item -LiteralPath $pub
    & $singleScript -AllowedSource '203.0.113.10/32' -WhatIf
    Assert-Equal (Test-Path -LiteralPath $pub) $false 'Script entry point required a separate public key'
    [IO.File]::WriteAllText($pub, "$key test-client", $utf8)

    # The default workflow needs no public-key file beside the copied script.
    Reset-MockOpenSSH -Client 'NotPresent'
    $script:postInstallState = 'NotPresent'
    Remove-Item -LiteralPath $pub
    Invoke-DayZSSHBootstrap -Key "$key embedded-client" -Sources '203.0.113.10/32' -Port 2222
    Assert-Equal $script:calls['zip-fallback'] 1 'Bootstrap did not recover from the exact silent Windows install failure'
    Assert-Equal $script:serviceRecord.PathName ('"' + (Join-Path $zipDir 'sshd.exe') + '"') 'Service does not run the selected fallback executable'
    Assert-Equal (Test-Path -LiteralPath $pub) $false 'Embedded bootstrap required an input key file'
    Assert-Equal ([IO.File]::ReadAllText((Join-Path $stateDir 'administrators_authorized_keys'))) "$key`r`n" 'Embedded key not authorized'
    [IO.File]::WriteAllText($pub, "$key test-client", $utf8)
    Assert-Equal $script:lastRule.LocalPort 2222 'Firewall port mismatch'
    Assert-Equal ($script:lastRule.RemoteAddress -join ',') '203.0.113.10/32' 'Firewall source mismatch'
    Assert-Equal $script:lastStartup 'Automatic' 'SSHD not automatic'
    $cfgPath = Join-Path $stateDir 'sshd_config'
    $keysPath = Join-Path $stateDir 'administrators_authorized_keys'
    $hostKey = Join-Path $stateDir 'ssh_host_ed25519_key'
    [IO.File]::WriteAllText($hostKey, 'existing host private key', $utf8)
    $beforeCfg = [IO.File]::ReadAllText($cfgPath)
    $beforeKeys = [IO.File]::ReadAllText($keysPath)
    Invoke-DayZSSHBootstrap -KeyPath $pub -Sources '203.0.113.10/32' -Port 2222
    Assert-Equal ([IO.File]::ReadAllText($cfgPath)) $beforeCfg 'Repeated bootstrap changed config'
    Assert-Equal ([IO.File]::ReadAllText($keysPath)) $beforeKeys 'Repeated bootstrap duplicated key'
    Assert-Equal ([IO.File]::ReadAllText($hostKey)) 'existing host private key' 'Host key was replaced'
    $beforeService = $script:serviceRecord.PSObject.Copy()
    $beforeMitigation = $script:mitigation

    foreach ($failure in @('failValidate', 'failRestart', 'failServiceInstall')) {
        Set-Variable -Scope Script -Name $failure -Value $true
        Assert-Throws { Invoke-DayZSSHBootstrap -KeyPath $pub -Sources '203.0.113.20/32' -Port 2022 } 'Injected failure was ignored'
        Assert-Equal ([IO.File]::ReadAllText($cfgPath)) $beforeCfg 'Rollback did not restore SSH config'
        Assert-Equal ([IO.File]::ReadAllText($keysPath)) $beforeKeys 'Rollback did not restore authorized keys'
        Assert-Equal $script:serviceRecord.StartMode $beforeService.StartMode 'Rollback did not restore startup mode'
        Assert-Equal $script:serviceRecord.PathName $beforeService.PathName 'Rollback did not restore service executable'
        Assert-Equal $script:mitigation $beforeMitigation 'Rollback did not restore mitigation state'
        Assert-Equal $script:managedRule.LocalPort 2222 'Rollback did not restore the previous scoped firewall rule'
        Assert-Equal ($script:managedRule.RemoteAddress -join ',') '203.0.113.10/32' 'Rollback changed the previous allowed network'
        Set-Variable -Scope Script -Name $failure -Value $false
    }
    Write-Host 'All mocked Windows SSH bootstrap tests passed.'
} finally {
    $env:WINDIR = $savedWindows
    $env:ProgramData = $savedData
    $env:ProgramFiles = $savedProgramFiles
    # Delete only the absolute, known test directory created above.
    $resolved = [IO.Path]::GetFullPath($taskRoot)
    $tempPrefix = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd('\') + '\'
    if (!$resolved.StartsWith($tempPrefix, [StringComparison]::OrdinalIgnoreCase) -or [IO.Path]::GetFileName($resolved) -notlike 'dzdeploy-ssh-test-*') { throw 'Unsafe test cleanup path' }
    if (Test-Path -LiteralPath $resolved) { Remove-Item -LiteralPath $resolved -Recurse -Force }
}
