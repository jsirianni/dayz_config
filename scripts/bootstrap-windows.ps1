#Requires -Version 5.1
<#
.SYNOPSIS
Prepares a Windows Server for dzdeploy. Run in an elevated local PowerShell.
.DESCRIPTION
The deployment client's public key is embedded below.
Copy only this script to each server.
.EXAMPLE
.\bootstrap-windows.ps1 -AllowedSource '203.0.113.10/32'
#>
[CmdletBinding(SupportsShouldProcess = $true, DefaultParameterSetName = 'Embedded')]
param(
    # Deployment client public key. Never embed the private key.
    [Parameter(ParameterSetName = 'Embedded')]
    [string]$PublicKey = 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGBS/OP1uRsxFiA0/tooHY+eNR2EkNsrdxGulSQtMVGn',
    [Parameter(ParameterSetName = 'File')]
    [string]$PublicKeyPath,
    [string[]]$AllowedSource,
    [string]$UserName = 'Administrator',
    [ValidateRange(1, 65535)][int]$SshPort = 22
)

function Test-DeploySource {
    param([string]$Source)
    $parts = $Source.Split('/')
    $address = $null
    if ($parts.Count -gt 2 -or ![Net.IPAddress]::TryParse($parts[0], [ref]$address)) { return $false }
    if ($parts.Count -eq 1) { return $true }
    $prefix = 0
    $maximum = 32
    if ($address.AddressFamily -eq [Net.Sockets.AddressFamily]::InterNetworkV6) { $maximum = 128 }
    return [int]::TryParse($parts[1], [ref]$prefix) -and $prefix -ge 1 -and $prefix -le $maximum
}

function ConvertTo-DeployPublicKey {
    param([string]$Text)
    if ([string]::IsNullOrWhiteSpace($Text) -or $Text.Trim() -notmatch '^(ssh-ed25519|ssh-rsa|ecdsa-sha2-nistp(?:256|384|521)) ([A-Za-z0-9+/]+={0,2})(?: [^\r\n]*)?$') {
        throw 'Embed exactly one OpenSSH public key in PublicKey, without authorized_keys options, or provide -PublicKeyPath.'
    }
    $kind = $Matches[1]
    $encoded = $Matches[2]
    try { $blob = [Convert]::FromBase64String($encoded) } catch { throw 'Invalid public key base64.' }
    if ($blob.Length -lt 8) { throw 'Invalid SSH public key.' }
    $length = ([int]$blob[0] * 16777216) + ([int]$blob[1] * 65536) + ([int]$blob[2] * 256) + [int]$blob[3]
    if ($length -ne $kind.Length -or $blob.Length -lt (4 + $length) -or [Text.Encoding]::ASCII.GetString($blob, 4, $length) -ne $kind) {
        throw 'Public key type does not match its SSH encoding.'
    }
    return "$kind $encoded"
}

function Merge-DeployAuthorizedKeys {
    param([string]$Existing, [string]$Key)
    $identity = ($Key -split ' ')[1]
    $lines = New-Object 'System.Collections.Generic.List[string]'
    $found = $false
    foreach ($line in ($Existing -split '\r?\n')) {
        if (!$line.Trim()) { continue }
        # Preserve unrelated keys, comments, and restricted keys verbatim.
        if ($line.Trim() -match ('^\S+ ' + [regex]::Escape($identity) + '(?: |$)')) {
            if ($found) { continue }
            $found = $true
        }
        $lines.Add($line)
    }
    if (!$found) { $lines.Add($Key) }
    return ($lines -join "`r`n") + "`r`n"
}

function New-DeploySshConfig {
    param([string]$Existing, [string]$Account, [int]$Port, [string]$SftpPath, [switch]$FileLogging)
    if ($Account -notmatch '^[a-zA-Z0-9_.-]+$') { throw 'UserName must be a simple local Windows account name.' }
    $begin = '# dzdeploy bootstrap begin'
    $end = '# dzdeploy bootstrap end'
    if (($Existing.Contains($begin)) -ne ($Existing.Contains($end))) { throw 'Incomplete managed SSH configuration block.' }
    $text = [regex]::Replace($Existing, '(?ms)^# dzdeploy bootstrap begin\r?\n.*?^# dzdeploy bootstrap end\r?\n?', '')
    $lines = $text -split '\r?\n'
    $global = New-Object 'System.Collections.Generic.List[string]'
    $matchLines = New-Object 'System.Collections.Generic.List[string]'
    $inMatch = $false
    $subsystem = $false
    foreach ($line in $lines) {
        if ($line -match '^\s*Match\s+') { $inMatch = $true }
        if ($inMatch) { $matchLines.Add($line); continue }
        if ($line -match '^\s*Port\s+') { continue }
        if ($FileLogging -and $line -match '^\s*SyslogFacility\s+') { continue }
        if ($line -match '^\s*Subsystem\s+sftp\s+') {
            if ($SftpPath) { continue }
            $subsystem = $true
        }
        $global.Add($line)
    }
    if ($SftpPath) {
        if (![IO.Path]::IsPathRooted($SftpPath) -or $SftpPath -match '["\r\n]') { throw 'SFTP executable must have a safe absolute path.' }
        $global.Add('Subsystem sftp "' + $SftpPath.Replace('\', '/') + '"')
    } elseif (!$subsystem) { $global.Add('Subsystem sftp sftp-server.exe') }
    if ($FileLogging) { $global.Add('SyslogFacility LOCAL0') }
    $managed = @(
        $begin,
        "Match User $($Account.ToLowerInvariant())",
        '    PubkeyAuthentication yes',
        '    AuthenticationMethods publickey',
        '    PasswordAuthentication no',
        '    AuthorizedKeysFile __PROGRAMDATA__/ssh/administrators_authorized_keys',
        '    ForceCommand none',
        '    AllowTcpForwarding no',
        '    PermitTTY yes',
        $end
    )
    # Place the account block before the stock administrators Match so its
    # authentication and command settings win the first-match precedence.
    return (@("Port $Port") + $global.ToArray() + $managed + $matchLines.ToArray() -join "`r`n").TrimEnd("`r", "`n") + "`r`n"
}

function Set-DeployPrivateAcl {
    param([string]$Path, [switch]$Directory)
    $acl = New-Object Security.AccessControl.FileSecurity
    if ($Directory) { $acl = New-Object Security.AccessControl.DirectorySecurity }
    $acl.SetAccessRuleProtection($true, $false)
    $admin = New-Object Security.Principal.SecurityIdentifier('S-1-5-32-544')
    $system = New-Object Security.Principal.SecurityIdentifier('S-1-5-18')
    $acl.SetOwner($admin)
    foreach ($sid in @($admin, $system)) {
        $inherit = [Security.AccessControl.InheritanceFlags]::None
        if ($Directory) { $inherit = [Security.AccessControl.InheritanceFlags]'ContainerInherit,ObjectInherit' }
        $rule = New-Object Security.AccessControl.FileSystemAccessRule($sid, 'FullControl', $inherit, 'None', 'Allow')
        $acl.AddAccessRule($rule)
    }
    Set-Acl -LiteralPath $Path -AclObject $acl
}

function Invoke-DeployNative {
    param([string]$Executable, [string[]]$ArgList)
    $output = & $Executable @ArgList 2>&1
    if ($LASTEXITCODE -ne 0) { throw "$Executable failed ($LASTEXITCODE): $output" }
    return $output
}

function Assert-DeployAdministrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    if (!$principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        throw 'Run this script from an elevated PowerShell terminal.'
    }
}

function Assert-DeployNativePowerShell {
    param(
        [bool]$Is64BitOperatingSystem = [Environment]::Is64BitOperatingSystem,
        [bool]$Is64BitProcess = [Environment]::Is64BitProcess
    )
    if ($Is64BitOperatingSystem -and !$Is64BitProcess) {
        $nativeShell = Join-Path $env:WINDIR 'Sysnative\WindowsPowerShell\v1.0\powershell.exe'
        throw "Run this script in elevated 64-bit Windows PowerShell, not Windows PowerShell (x86). From a 32-bit shell, run & '$nativeShell', then rerun this script. The 32-bit shell redirects System32 and cannot load the LocalAccounts module."
    }
}

function Get-DeployOpenSSHZipSpec {
    return [pscustomobject]@{
        Version = '10.4.0.0p1-Preview'
        Url = 'https://github.com/PowerShell/Win32-OpenSSH/releases/download/10.4.0.0p1-Preview/OpenSSH-Win64.zip'
        SHA256 = 'b67ff8becd203502190ec42ac7787c24c2cd3325ed7700485fe7a1c83d2a7957'
    }
}

function Set-DeployOpenSSHDirectoryAcl {
    param([string]$Path)
    $acl = New-Object Security.AccessControl.DirectorySecurity
    $acl.SetAccessRuleProtection($true, $false)
    $admin = New-Object Security.Principal.SecurityIdentifier('S-1-5-32-544')
    $acl.SetOwner($admin)
    foreach ($identity in @('S-1-5-32-544', 'S-1-5-18', 'S-1-5-11')) {
        $rights = [Security.AccessControl.FileSystemRights]::FullControl
        if ($identity -eq 'S-1-5-11') { $rights = [Security.AccessControl.FileSystemRights]::ReadAndExecute }
        $sid = New-Object Security.Principal.SecurityIdentifier($identity)
        $rule = New-Object Security.AccessControl.FileSystemAccessRule($sid, $rights, 'ContainerInherit,ObjectInherit', 'None', 'Allow')
        $acl.AddAccessRule($rule)
    }
    Set-Acl -LiteralPath $Path -AclObject $acl
}

function Assert-DeployNoReparsePoint {
    param([string]$Path)
    if (Test-Path -LiteralPath $Path) {
        if (((Get-Item -LiteralPath $Path -Force).Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
            throw "OpenSSH package path must not be a junction or symbolic link: $Path"
        }
    }
}

function Assert-DeployOpenSSHArchiveHash {
    param([string]$Path, [string]$ExpectedSHA256)
    if ((Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash -ine $ExpectedSHA256) {
        throw 'Microsoft OpenSSH archive SHA-256 verification failed.'
    }
}

function Get-DeployOpenSSHArchive {
    param([uri]$Uri, [string]$Destination, [string]$ExpectedSHA256)
    $ErrorActionPreference = 'Stop'
    $maximumBytes = 16MB
    $savedProtocol = [Net.ServicePointManager]::SecurityProtocol
    $response = $null
    $output = $null
    try {
        [Net.ServicePointManager]::SecurityProtocol = $savedProtocol -bor [Net.SecurityProtocolType]::Tls12
        for ($redirect = 0; ; $redirect++) {
            if ($Uri.Scheme -ne 'https' -or $Uri.UserInfo -or $redirect -gt 5) { throw 'Unsafe OpenSSH download URL or redirect.' }
            $request = [Net.HttpWebRequest]::Create($Uri)
            $request.AllowAutoRedirect = $false
            $request.Timeout = 120000
            $request.ReadWriteTimeout = 30000
            $request.UserAgent = 'DayZDeploy-bootstrap'
            $response = $request.GetResponse()
            $status = [int]$response.StatusCode
            if ($status -in @(301, 302, 303, 307, 308)) {
                $location = $response.Headers['Location']
                if (!$location) { throw 'OpenSSH download redirect has no destination.' }
                $Uri = New-Object Uri($Uri, $location)
                $response.Dispose()
                $response = $null
                continue
            }
            if ($status -ne 200) { throw "OpenSSH download returned HTTP $status." }
            if ($response.ContentLength -gt $maximumBytes) { throw 'OpenSSH download exceeds the size limit.' }
            $inputStream = $response.GetResponseStream()
            $output = [IO.File]::Open($Destination, [IO.FileMode]::CreateNew, [IO.FileAccess]::Write, [IO.FileShare]::None)
            $buffer = New-Object byte[] 65536
            $total = 0L
            try {
                while (($count = $inputStream.Read($buffer, 0, $buffer.Length)) -gt 0) {
                    $total += $count
                    if ($total -gt $maximumBytes) { throw 'OpenSSH download exceeds the size limit.' }
                    $output.Write($buffer, 0, $count)
                }
            } finally { $inputStream.Dispose() }
            $output.Dispose()
            $output = $null
            if ($total -eq 0) { throw 'OpenSSH download was empty.' }
            break
        }
        Assert-DeployOpenSSHArchiveHash -Path $Destination -ExpectedSHA256 $ExpectedSHA256
    } finally {
        if ($output) { $output.Dispose() }
        if ($response) { $response.Dispose() }
        [Net.ServicePointManager]::SecurityProtocol = $savedProtocol
    }
}

function Expand-DeployOpenSSHArchive {
    param([string]$ArchivePath, [string]$Destination)
    $ErrorActionPreference = 'Stop'
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    if (Test-Path -LiteralPath $Destination) {
        Assert-DeployNoReparsePoint $Destination
        if (@(Get-ChildItem -LiteralPath $Destination -Force).Count -gt 0) { throw 'OpenSSH extraction directory must be empty.' }
    } else { New-Item -ItemType Directory -Path $Destination | Out-Null }
    $destinationRoot = [IO.Path]::GetFullPath($Destination).TrimEnd('\') + '\'
    $archive = [IO.Compression.ZipFile]::OpenRead($ArchivePath)
    $expandedBytes = 0L
    try {
        foreach ($entry in $archive.Entries) {
            $name = $entry.FullName.Replace('\', '/')
            $unixType = ($entry.ExternalAttributes -shr 16) -band 61440
            if ($name -notlike 'OpenSSH-Win64/*' -or $name.Contains(':') -or $name.Contains([char]0) -or $unixType -eq 40960 -or ($entry.ExternalAttributes -band 1024) -ne 0) {
                throw "Unsafe OpenSSH archive entry: $name"
            }
            $relative = $name.Substring('OpenSSH-Win64/'.Length)
            if (!$relative) { continue }
            $parts = $relative.TrimEnd('/').Split('/')
            if (@($parts | Where-Object {
                !$_ -or $_ -eq '.' -or $_ -eq '..' -or $_ -cne $_.TrimEnd(' ', '.') -or
                $_.IndexOfAny([IO.Path]::GetInvalidFileNameChars()) -ge 0 -or
                $_ -match '^(CON|PRN|AUX|NUL|COM[1-9]|LPT[1-9])(?:\.|$)'
            }).Count -gt 0) { throw "Unsafe OpenSSH archive entry: $name" }
            $target = [IO.Path]::GetFullPath((Join-Path $Destination ($relative.Replace('/', '\'))))
            if (!$target.StartsWith($destinationRoot, [StringComparison]::OrdinalIgnoreCase)) { throw "OpenSSH archive entry escapes extraction directory: $name" }
            if ($entry.Length -gt 128MB -or ($expandedBytes + $entry.Length) -gt 128MB) { throw 'OpenSSH archive exceeds the expanded size limit.' }
            if ($name.EndsWith('/')) {
                New-Item -ItemType Directory -Force -Path $target | Out-Null
                continue
            }
            New-Item -ItemType Directory -Force -Path ([IO.Path]::GetDirectoryName($target)) | Out-Null
            $inputStream = $entry.Open()
            $output = [IO.File]::Open($target, [IO.FileMode]::CreateNew, [IO.FileAccess]::Write, [IO.FileShare]::None)
            try {
                $buffer = New-Object byte[] 65536
                while (($count = $inputStream.Read($buffer, 0, $buffer.Length)) -gt 0) {
                    $expandedBytes += $count
                    if ($expandedBytes -gt 128MB) { throw 'OpenSSH archive exceeds the expanded size limit.' }
                    $output.Write($buffer, 0, $count)
                }
            } finally { $output.Dispose(); $inputStream.Dispose() }
        }
    } finally { $archive.Dispose() }
}

function Assert-DeployMicrosoftSignature {
    param([string]$Path)
    $signature = Get-AuthenticodeSignature -LiteralPath $Path
    if ($signature.Status -ne 'Valid' -or !$signature.SignerCertificate -or $signature.SignerCertificate.Subject -notmatch '(?:^|,\s*)O=Microsoft Corporation(?:,|$)') {
        throw "OpenSSH Microsoft signature verification failed: $Path ($($signature.Status))"
    }
}

function Assert-DeployOpenSSHZipPayload {
    param([string]$Directory)
    Assert-DeployNoReparsePoint $Directory
    foreach ($required in @('sshd.exe', 'ssh-keygen.exe', 'sftp-server.exe', 'sshd_config_default', 'sshd-auth.exe', 'sshd-session.exe', 'libcrypto.dll')) {
        $path = Join-Path $Directory $required
        if (!(Test-Path -LiteralPath $path -PathType Leaf) -or (Get-Item -LiteralPath $path).Length -eq 0) { throw "Microsoft OpenSSH package is missing $required." }
    }
    foreach ($item in @(Get-ChildItem -LiteralPath $Directory -Recurse -Force)) {
        Assert-DeployNoReparsePoint $item.FullName
        if (!$item.PSIsContainer -and $item.Extension -in @('.exe', '.dll')) { Assert-DeployMicrosoftSignature $item.FullName }
    }
}

function Test-DeployOpenSSHZipInstallation {
    param([string]$Directory)
    try {
        $spec = Get-DeployOpenSSHZipSpec
        Assert-DeployOpenSSHZipPayload $Directory
        $manifestPath = Join-Path $Directory 'dzdeploy-package.json'
        $manifest = [IO.File]::ReadAllText($manifestPath) | ConvertFrom-Json
        if ($manifest.Version -cne $spec.Version -or $manifest.ArchiveSHA256 -ine $spec.SHA256) { return $false }
        $root = [IO.Path]::GetFullPath($Directory).TrimEnd('\') + '\'
        $files = @(Get-ChildItem -LiteralPath $Directory -Recurse -File -Force | Where-Object { $_.FullName -ine $manifestPath })
        if (@($manifest.Files).Count -ne $files.Count) { return $false }
        $seen = @{}
        foreach ($file in $manifest.Files) {
            if (!$file.Path -or $file.Path.Contains(':') -or [IO.Path]::IsPathRooted($file.Path)) { return $false }
            $path = [IO.Path]::GetFullPath((Join-Path $Directory $file.Path))
            if ($seen.ContainsKey($path) -or !$path.StartsWith($root, [StringComparison]::OrdinalIgnoreCase) -or !(Test-Path -LiteralPath $path -PathType Leaf)) { return $false }
            if ((Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash -ine $file.SHA256) { return $false }
            $seen[$path] = $true
        }
        return $true
    } catch { Write-Verbose "OpenSSH package validation failed: $($_.Exception.Message)"; return $false }
}

function Install-DeployOpenSSHZip {
    $ErrorActionPreference = 'Stop'
    $spec = Get-DeployOpenSSHZipSpec
    $base = [IO.Path]::GetFullPath((Join-Path $env:ProgramFiles 'DayZDeploy'))
    Assert-DeployNoReparsePoint $base
    New-Item -ItemType Directory -Force -Path $base | Out-Null
    Set-DeployOpenSSHDirectoryAcl $base
    $parent = Join-Path $base 'OpenSSH'
    Assert-DeployNoReparsePoint $parent
    New-Item -ItemType Directory -Force -Path $parent | Out-Null
    Set-DeployOpenSSHDirectoryAcl $parent
    $installed = Join-Path $parent $spec.Version
    if (Test-Path -LiteralPath $installed) {
        if (!(Test-DeployOpenSSHZipInstallation $installed)) { throw "Existing managed OpenSSH package is incomplete or corrupt: $installed. It was not overwritten." }
    } else {
        $stage = Join-Path $parent ('.stage-' + [Guid]::NewGuid().ToString('N'))
        New-Item -ItemType Directory -Path $stage | Out-Null
        Set-DeployOpenSSHDirectoryAcl $stage
        try {
            $archivePath = Join-Path $stage 'OpenSSH-Win64.zip'
            Write-Host "Downloading verified Microsoft OpenSSH $($spec.Version)..."
            Get-DeployOpenSSHArchive -Uri $spec.Url -Destination $archivePath -ExpectedSHA256 $spec.SHA256
            # Verify again at the extraction boundary, including mocked/cached downloaders.
            Assert-DeployOpenSSHArchiveHash -Path $archivePath -ExpectedSHA256 $spec.SHA256
            $payload = Join-Path $stage 'payload'
            Expand-DeployOpenSSHArchive -ArchivePath $archivePath -Destination $payload
            Assert-DeployOpenSSHZipPayload $payload
            $payloadRoot = [IO.Path]::GetFullPath($payload).TrimEnd('\') + '\'
            $inventory = @(Get-ChildItem -LiteralPath $payload -Recurse -File -Force | Sort-Object FullName | ForEach-Object {
                [pscustomobject]@{ Path = $_.FullName.Substring($payloadRoot.Length).Replace('\', '/'); SHA256 = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant() }
            })
            $manifest = [pscustomobject]@{ Version = $spec.Version; ArchiveSHA256 = $spec.SHA256; Files = $inventory }
            [IO.File]::WriteAllText((Join-Path $payload 'dzdeploy-package.json'), ($manifest | ConvertTo-Json -Depth 4), (New-Object Text.UTF8Encoding($false)))
            Set-DeployOpenSSHDirectoryAcl $payload
            if (!(Test-DeployOpenSSHZipInstallation $payload)) { throw 'Staged Microsoft OpenSSH package failed verification.' }
            $parentRoot = [IO.Path]::GetFullPath($parent).TrimEnd('\') + '\'
            if (!$payloadRoot.StartsWith($parentRoot, [StringComparison]::OrdinalIgnoreCase) -or ![IO.Path]::GetFullPath($installed).StartsWith($parentRoot, [StringComparison]::OrdinalIgnoreCase)) { throw 'Unsafe OpenSSH installation path.' }
            Move-Item -LiteralPath $payload -Destination $installed
        } finally {
            $resolvedStage = [IO.Path]::GetFullPath($stage)
            $parentRoot = [IO.Path]::GetFullPath($parent).TrimEnd('\') + '\'
            if (!$resolvedStage.StartsWith($parentRoot, [StringComparison]::OrdinalIgnoreCase) -or [IO.Path]::GetFileName($resolvedStage) -notmatch '^\.stage-[a-f0-9]{32}$') { throw 'Unsafe OpenSSH staging cleanup path.' }
            Assert-DeployNoReparsePoint $resolvedStage
            if (Test-Path -LiteralPath $resolvedStage) { Remove-Item -LiteralPath $resolvedStage -Recurse -Force }
        }
    }
    return [pscustomobject]@{ Directory = $installed; Provider = 'MicrosoftZip'; Version = $spec.Version; ServiceName = 'sshd' }
}

function Get-DeploySshService {
    Get-CimInstance -ClassName Win32_Service -Filter "Name='sshd'" -ErrorAction Stop
}

function Get-DeploySshExecutable {
    param($Service)
    if (!$Service) { return $null }
    if ($Service.PathName -notmatch '^\s*(?:"(?<exe>[^"\r\n]+\\sshd\.exe)"|(?<exe>[^"\r\n]+?\\sshd\.exe))\s*$') {
        throw 'The existing sshd service has an unsupported executable or custom arguments. Its configuration was preserved.'
    }
    return [IO.Path]::GetFullPath($Matches['exe'])
}

function Test-DeploySshPayload {
    param([string]$Directory)
    foreach ($file in @('sshd.exe', 'ssh-keygen.exe', 'sftp-server.exe', 'sshd_config_default')) {
        if (!(Test-Path -LiteralPath (Join-Path $Directory $file) -PathType Leaf)) { return $false }
    }
    return $true
}

function New-DeploySshInstallation {
    param([string]$Directory, [string]$Provider = 'Existing')
    $version = [Diagnostics.FileVersionInfo]::GetVersionInfo((Join-Path $Directory 'sshd.exe')).FileVersion
    return [pscustomobject]@{ Directory = $Directory; Provider = $Provider; Version = $version; ServiceName = 'sshd' }
}

function Install-DeployOpenSSH {
    $ErrorActionPreference = 'Stop'
    $service = Get-DeploySshService
    $executable = Get-DeploySshExecutable $service
    if ($executable) {
        $directory = Split-Path -Parent $executable
        if (Test-DeploySshPayload $directory) {
            $managedDirectory = Join-Path $env:ProgramFiles 'DayZDeploy\OpenSSH\10.4.0.0p1-Preview'
            if ([IO.Path]::GetFullPath($directory) -ieq [IO.Path]::GetFullPath($managedDirectory)) {
                return Install-DeployOpenSSHZip
            }
            return New-DeploySshInstallation -Directory $directory
        }
    }
    $failures = New-Object 'System.Collections.Generic.List[string]'
    foreach ($name in @('OpenSSH.Client~~~~0.0.1.0', 'OpenSSH.Server~~~~0.0.1.0')) {
        $pending = $false
        try {
            $feature = Get-WindowsCapability -Online -Name $name -ErrorAction Stop
            if ([string]$feature.State -match 'Pending') { $pending = $true; throw "Windows reports $name as $($feature.State)." }
            if ($feature.State -ne 'Installed') {
                Write-Host "Installing $name..."
                $installed = Add-WindowsCapability -Online -Name $name -ErrorAction Stop
                if ($installed.RestartNeeded) { $pending = $true; throw "Windows reports that $name requires a reboot." }
                $feature = Get-WindowsCapability -Online -Name $name -ErrorAction Stop
                if ([string]$feature.State -match 'Pending') { $pending = $true; throw "Windows reports $name as $($feature.State)." }
            }
            if ($feature.State -ne 'Installed') { $failures.Add("$name remains $($feature.State)") }
        } catch {
            if ($pending) { throw "$($_.Exception.Message) Reboot the server to finish servicing, then rerun this same script." }
            $failures.Add("${name}: $($_.Exception.Message)")
        }
    }
    $inboxDirectory = Join-Path $env:WINDIR 'System32\OpenSSH'
    if (Test-DeploySshPayload $inboxDirectory) { return New-DeploySshInstallation -Directory $inboxDirectory -Provider 'WindowsCapability' }
    Write-Host "Windows OpenSSH is unavailable ($($failures -join '; ')). Installing the verified Microsoft package..."
    return Install-DeployOpenSSHZip
}

function Get-DeploySshFileSnapshot {
    param([string]$Path)
    $exists = Test-Path -LiteralPath $Path -PathType Leaf
    $bytes = $null; $acl = $null
    if ($exists) { $bytes = [IO.File]::ReadAllBytes($Path); $acl = Get-Acl -LiteralPath $Path }
    return [pscustomobject]@{ Path = $Path; Exists = $exists; Bytes = $bytes; Acl = $acl }
}

function Restore-DeploySshFileSnapshot {
    param($Snapshot)
    if ($Snapshot.Exists) {
        if (Test-Path -LiteralPath $Snapshot.Path -PathType Leaf) {
            $sameBytes = [Convert]::ToBase64String([IO.File]::ReadAllBytes($Snapshot.Path)) -ceq [Convert]::ToBase64String($Snapshot.Bytes)
            if ($sameBytes -and (Get-Acl -LiteralPath $Snapshot.Path).Sddl -ceq $Snapshot.Acl.Sddl) { return }
        }
        [IO.File]::WriteAllBytes($Snapshot.Path, $Snapshot.Bytes)
        Set-Acl -LiteralPath $Snapshot.Path -AclObject $Snapshot.Acl
    } elseif (Test-Path -LiteralPath $Snapshot.Path) { Remove-Item -LiteralPath $Snapshot.Path -Force }
}

function Set-DeploySshServiceInstallation {
    param($Installation, $PreviousService)
    $current = Get-DeploySshService
    $target = Join-Path $Installation.Directory 'sshd.exe'
    if ($current) {
        if ($current.StartName -notmatch '^(LocalSystem|NT AUTHORITY\\SYSTEM)$') { throw 'Existing sshd must use LocalSystem; its service account was preserved.' }
        if ((Get-DeploySshExecutable $current) -ine $target) {
            Stop-Service -Name sshd -ErrorAction Stop
            $result = Invoke-CimMethod -InputObject $current -MethodName Change -Arguments @{ PathName = '"' + $target + '"' } -ErrorAction Stop
            if ($result.ReturnValue -ne 0) { throw "Changing sshd executable failed: $($result.ReturnValue)" }
        }
    } else {
        $created = New-Service -Name sshd -DisplayName 'OpenSSH SSH Server' -BinaryPathName ('"' + $target + '"') -StartupType Manual -ErrorAction Stop
        if ($created -is [IDisposable]) { $created.Dispose() }
        Invoke-DeployNative (Join-Path $env:WINDIR 'System32\sc.exe') @('privs', 'sshd', 'SeAssignPrimaryTokenPrivilege/SeTcbPrivilege/SeBackupPrivilege/SeRestorePrivilege/SeImpersonatePrivilege') | Out-Null
    }
}

function Restore-DeploySshService {
    param($PreviousService)
    $current = Get-DeploySshService
    if (!$PreviousService) {
        if ($current) {
            Stop-Service -Name sshd -ErrorAction Stop
            Invoke-DeployNative (Join-Path $env:WINDIR 'System32\sc.exe') @('delete', 'sshd') | Out-Null
        }
        return
    }
    if (!$current) { throw 'Original sshd service disappeared during bootstrap.' }
    if ($current.PathName -cne $PreviousService.PathName) {
        Stop-Service -Name sshd -ErrorAction Stop
        $result = Invoke-CimMethod -InputObject $current -MethodName Change -Arguments @{ PathName = $PreviousService.PathName } -ErrorAction Stop
        if ($result.ReturnValue -ne 0) { throw "Restoring sshd executable failed: $($result.ReturnValue)" }
    }
    $start = $PreviousService.StartMode
    if ($start -eq 'Auto') { $start = 'Automatic' }
    if ($PreviousService.State -eq 'Running') {
        Set-Service -Name sshd -StartupType Manual
        Restart-Service sshd
    } else { Stop-Service sshd }
    Set-Service -Name sshd -StartupType $start
}

function Get-DeployFirewallRuleSnapshot {
    param([string]$Name)
    $rule = Get-NetFirewallRule -Name $Name -ErrorAction SilentlyContinue
    if (!$rule) { return $null }
    $port = $rule | Get-NetFirewallPortFilter
    $address = $rule | Get-NetFirewallAddressFilter
    $application = $rule | Get-NetFirewallApplicationFilter
    return [pscustomobject]@{
        Name = $Name; DisplayName = $rule.DisplayName; Enabled = [string]$rule.Enabled
        Direction = [string]$rule.Direction; Action = [string]$rule.Action; Profile = [string]$rule.Profile
        Protocol = [string]$port.Protocol; LocalPort = $port.LocalPort
        RemoteAddress = $address.RemoteAddress; Program = $application.Program
    }
}

function Restore-DeployFirewallRuleSnapshot {
    param([string]$Name, $Snapshot)
    $current = Get-DeployFirewallRuleSnapshot $Name
    if (!$current -and !$Snapshot) { return }
    if (($current | ConvertTo-Json -Depth 4 -Compress) -ceq ($Snapshot | ConvertTo-Json -Depth 4 -Compress)) { return }
    if ($current) { Remove-NetFirewallRule -Name $Name }
    if ($Snapshot) {
        New-NetFirewallRule -Name $Snapshot.Name -DisplayName $Snapshot.DisplayName -Enabled $Snapshot.Enabled -Direction $Snapshot.Direction -Action $Snapshot.Action -Profile $Snapshot.Profile -Protocol $Snapshot.Protocol -LocalPort $Snapshot.LocalPort -RemoteAddress $Snapshot.RemoteAddress -Program $Snapshot.Program | Out-Null
    }
}

function Set-DeploySshMitigation {
    $path = 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Image File Execution Options\sshd.exe'
    $keyExisted = Test-Path -LiteralPath $path
    $old = Get-ItemProperty -LiteralPath $path -Name MitigationOptions -ErrorAction SilentlyContinue
    $snapshot = [pscustomobject]@{ Path = $path; KeyExisted = $keyExisted; Exists = $null -ne $old; Value = $old.MitigationOptions }
    try {
        if (!$old) {
            if (!$keyExisted) { New-Item -Path $path -Force | Out-Null }
            $value = New-Object byte[] 19
            $value[18] = 0x10
            New-ItemProperty -LiteralPath $path -Name MitigationOptions -PropertyType Binary -Value $value -ErrorAction Stop | Out-Null
        }
    } catch { Restore-DeploySshMitigation $snapshot; throw }
    return $snapshot
}

function Restore-DeploySshMitigation {
    param($Snapshot)
    if (!$Snapshot) { return }
    if ($Snapshot.Exists) { Set-ItemProperty -LiteralPath $Snapshot.Path -Name MitigationOptions -Value $Snapshot.Value }
    else {
        Remove-ItemProperty -LiteralPath $Snapshot.Path -Name MitigationOptions -ErrorAction SilentlyContinue
        if (!$Snapshot.KeyExisted -and (Test-Path -LiteralPath $Snapshot.Path)) { Remove-Item -LiteralPath $Snapshot.Path -ErrorAction Stop }
    }
}

function Invoke-DayZSSHBootstrap {
    [CmdletBinding(SupportsShouldProcess = $true)]
    param([string]$KeyPath, [string]$Key, [string[]]$Sources, [string]$Account = 'Administrator', [int]$Port = 22)
    $ErrorActionPreference = 'Stop'
    Assert-DeployNativePowerShell
    Assert-DeployAdministrator
    if ($Account -notmatch '^[a-zA-Z0-9_.-]+$') { throw 'UserName must be a simple local Windows account name.' }
    if ($KeyPath -and $Key) { throw 'Provide either an embedded public key or -PublicKeyPath, not both.' }
    if ($KeyPath) {
        if (!(Test-Path -LiteralPath $KeyPath -PathType Leaf)) { throw 'PublicKeyPath must point to the client public key.' }
        $Key = [IO.File]::ReadAllText((Resolve-Path -LiteralPath $KeyPath))
    }
    if (!$Sources -or @($Sources | Where-Object { !(Test-DeploySource $_) }).Count -gt 0) { throw 'Provide -AllowedSource as explicit IP addresses or CIDRs; unrestricted /0 is not allowed.' }
    if ($Port -lt 1 -or $Port -gt 65535) { throw 'SSH port must be between 1 and 65535.' }
    $key = ConvertTo-DeployPublicKey $Key
    $user = Get-LocalUser -Name $Account
    if (!$user.Enabled) { throw 'The SSH account must already be enabled.' }
    $admins = @(Get-LocalGroupMember -SID 'S-1-5-32-544')
    if ($user.SID.Value -notin @($admins | ForEach-Object { $_.SID.Value })) { throw 'The SSH account must be a local Administrator.' }
    if (!$PSCmdlet.ShouldProcess($env:COMPUTERNAME, "Configure key-only SSH for $Account on TCP $Port from $($Sources -join ',')")) { return }

    $previousService = Get-DeploySshService
    if ($previousService -and $previousService.StartName -notmatch '^(LocalSystem|NT AUTHORITY\\SYSTEM)$') { throw 'Existing sshd must use LocalSystem; its service account was preserved.' }
    $root = Join-Path $env:ProgramData 'ssh'
    $rootAcl = $null
    if (Test-Path -LiteralPath $root) { $rootAcl = Get-Acl -LiteralPath $root }
    $cfgPath = Join-Path $root 'sshd_config'
    $keysPath = Join-Path $root 'administrators_authorized_keys'
    $snapshots = @((Get-DeploySshFileSnapshot $cfgPath), (Get-DeploySshFileSnapshot $keysPath))
    if (Test-Path -LiteralPath $root) {
        $snapshots += @(Get-ChildItem -LiteralPath $root -Filter 'ssh_host_*_key*' -File | ForEach-Object { Get-DeploySshFileSnapshot $_.FullName })
    }
    $ruleName = 'DayZDeploy-SSH'
    $oldRule = Get-NetFirewallRule -Name $ruleName -ErrorAction SilentlyContinue
    $oldPort = $null; $oldAddress = $null; $oldApplication = $null
    if ($oldRule) { $oldPort = $oldRule | Get-NetFirewallPortFilter; $oldAddress = $oldRule | Get-NetFirewallAddressFilter; $oldApplication = $oldRule | Get-NetFirewallApplicationFilter }
    $broad = @{}
    foreach ($name in @('OpenSSH-Server-In-TCP', 'sshd')) { $broad[$name] = Get-DeployFirewallRuleSnapshot $name }
    $firewallChanged = $false; $serviceChanged = $false; $mitigation = $null; $backup = $null
    try {
        $installation = Install-DeployOpenSSH
        $sshd = Join-Path $installation.Directory 'sshd.exe'
        $keygen = Join-Path $installation.Directory 'ssh-keygen.exe'
        $utf8 = New-Object Text.UTF8Encoding($false)
        New-Item -ItemType Directory -Force -Path $root | Out-Null
        Set-DeployPrivateAcl $root -Directory
        $backup = Join-Path $root ('dzdeploy-backup-' + [DateTime]::UtcNow.ToString('yyyyMMddTHHmmssfffffff'))
        New-Item -ItemType Directory -Path $backup | Out-Null
        Set-DeployPrivateAcl $backup -Directory
        foreach ($snapshot in $snapshots) {
            if ($snapshot.Exists) { [IO.File]::WriteAllBytes((Join-Path $backup ([IO.Path]::GetFileName($snapshot.Path))), $snapshot.Bytes) }
        }
        if ($previousService) { $previousService | Select-Object PathName, StartMode, State, StartName | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $backup 'service.json') -Encoding UTF8 }
        Invoke-DeployNative $keygen @('-A') | Out-Null
        Get-ChildItem -LiteralPath $root -Filter 'ssh_host_*_key' -File | ForEach-Object { Set-DeployPrivateAcl $_.FullName }
        $oldCfg = [IO.File]::ReadAllText((Join-Path $installation.Directory 'sshd_config_default'))
        if ($snapshots[0].Exists) { $oldCfg = $utf8.GetString($snapshots[0].Bytes).TrimStart([char]0xfeff) }
        $oldKeys = ''
        if ($snapshots[1].Exists) { $oldKeys = $utf8.GetString($snapshots[1].Bytes).TrimStart([char]0xfeff) }
        $newCfg = New-DeploySshConfig -Existing $oldCfg -Account $Account -Port $Port -SftpPath (Join-Path $installation.Directory 'sftp-server.exe') -FileLogging:($installation.Provider -eq 'MicrosoftZip')
        $newKeys = Merge-DeployAuthorizedKeys $oldKeys $key
        $keyCandidate = Join-Path $backup 'client-public-key.pub'
        [IO.File]::WriteAllText($keyCandidate, $key, $utf8)
        Invoke-DeployNative $keygen @('-lf', $keyCandidate, '-E', 'sha256') | Out-Null
        $candidate = Join-Path $backup 'sshd_config.candidate'
        [IO.File]::WriteAllText($candidate, $newCfg, $utf8)
        Invoke-DeployNative $sshd @('-t', '-f', $candidate) | Out-Null
        foreach ($path in @($cfgPath, $keysPath)) {
            if (!(Test-Path -LiteralPath $path)) { [IO.File]::WriteAllBytes($path, [byte[]]@()) }
            Set-DeployPrivateAcl $path
        }
        [IO.File]::WriteAllText($keysPath, $newKeys, $utf8)
        [IO.File]::WriteAllText($cfgPath, $newCfg, $utf8)
        if ($installation.Provider -eq 'MicrosoftZip') { New-Item -ItemType Directory -Force -Path (Join-Path $root 'logs') | Out-Null }
        Invoke-DeployNative $sshd @('-t', '-f', $cfgPath) | Out-Null
        $firewallChanged = $true
        if ($oldRule) { Remove-NetFirewallRule -Name $ruleName }
        New-NetFirewallRule -Name $ruleName -DisplayName 'DayZ Deploy SSH' -Direction Inbound -Action Allow -Protocol TCP -LocalPort $Port -RemoteAddress $Sources -Program $sshd -Profile Any | Out-Null
        foreach ($name in @('OpenSSH-Server-In-TCP', 'sshd')) {
            Get-NetFirewallRule -Name $name -ErrorAction SilentlyContinue | Disable-NetFirewallRule | Out-Null
        }
        if ($installation.Provider -eq 'MicrosoftZip') { $mitigation = Set-DeploySshMitigation }
        $serviceChanged = $true
        Set-DeploySshServiceInstallation -Installation $installation -PreviousService $previousService
        Set-Service sshd -StartupType Automatic
        $current = Get-DeploySshService
        if ($current.State -eq 'Running') { Restart-Service sshd } else { Start-Service sshd }
        $controller = Get-Service sshd
        try { $controller.WaitForStatus('Running', [TimeSpan]::FromSeconds(30)) } finally { if ($controller -is [IDisposable]) { $controller.Dispose() } }
        $fingerprint = Invoke-DeployNative $keygen @('-lf', (Join-Path $root 'ssh_host_ed25519_key.pub'), '-E', 'sha256')
        Write-Host "SSH ready: user=$Account port=$Port sources=$($Sources -join ',') provider=$($installation.Provider) version=$($installation.Version)"
        Write-Host "Trusted host fingerprint: $fingerprint"
        Write-Host "Next, on the client: dzdeploy deploy <target> --restart --host-key <SHA256:fingerprint-above>"
        Write-Host "SSH backup: $backup"
    } catch {
        $failure = $_
        if (Test-Path -LiteralPath $root) {
            foreach ($snapshot in $snapshots) {
                try { Restore-DeploySshFileSnapshot $snapshot } catch { Write-Warning "File rollback failed for $($snapshot.Path): $_" }
            }
            foreach ($file in @(Get-ChildItem -LiteralPath $root -Filter 'ssh_host_*_key*' -File)) {
                if ($file.Name -match '^ssh_host_(rsa|ecdsa|ed25519|dsa)_key(?:\.pub)?$' -and $file.FullName -notin @($snapshots.Path)) {
                    try { Remove-Item -LiteralPath $file.FullName -Force } catch { Write-Warning "Host key rollback failed: $_" }
                }
            }
        }
        if ($firewallChanged) {
            try {
                Get-NetFirewallRule -Name $ruleName -ErrorAction SilentlyContinue | Remove-NetFirewallRule
                if ($oldRule) { New-NetFirewallRule -Name $ruleName -DisplayName $oldRule.DisplayName -Enabled $oldRule.Enabled -Direction $oldRule.Direction -Action $oldRule.Action -Protocol $oldPort.Protocol -LocalPort $oldPort.LocalPort -RemoteAddress $oldAddress.RemoteAddress -Program $oldApplication.Program -Profile $oldRule.Profile | Out-Null }
            } catch { Write-Warning "Firewall rollback failed: $_" }
        }
        # Windows capability installation can itself create a service or broad
        # firewall rule, before our final configuration phase has begun.
        foreach ($name in @('OpenSSH-Server-In-TCP', 'sshd')) {
            try { Restore-DeployFirewallRuleSnapshot -Name $name -Snapshot $broad[$name] } catch { Write-Warning "Firewall rollback failed for ${name}: $_" }
        }
        try { Restore-DeploySshMitigation $mitigation } catch { Write-Warning "Mitigation rollback failed: $_" }
        try {
            $currentService = Get-DeploySshService
            $serviceDiffers = ($null -eq $currentService) -ne ($null -eq $previousService)
            if ($currentService -and $previousService) {
                foreach ($property in @('PathName', 'StartMode', 'State', 'StartName')) {
                    if ($currentService.$property -cne $previousService.$property) { $serviceDiffers = $true }
                }
            }
            if ($serviceChanged -or $serviceDiffers) { Restore-DeploySshService $previousService }
        } catch { Write-Warning "Service rollback failed: $_" }
        if ($rootAcl) {
            try { if ((Get-Acl -LiteralPath $root).Sddl -cne $rootAcl.Sddl) { Set-Acl -LiteralPath $root -AclObject $rootAcl } } catch { Write-Warning "Directory ACL rollback failed: $_" }
        }
        throw $failure
    }
}

# Dot-sourcing exposes pure helpers and the orchestrator for mocked tests.
if ($MyInvocation.InvocationName -ne '.') {
    if ($PSCmdlet.ParameterSetName -eq 'File') {
        Invoke-DayZSSHBootstrap -KeyPath $PublicKeyPath -Sources $AllowedSource -Account $UserName -Port $SshPort
    } else {
        Invoke-DayZSSHBootstrap -Key $PublicKey -Sources $AllowedSource -Account $UserName -Port $SshPort
    }
}
