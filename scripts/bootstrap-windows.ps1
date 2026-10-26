#Requires -Version 5.1
<#
.SYNOPSIS
Prepares a Windows Server for dzdeploy. Run in an elevated local PowerShell.
.DESCRIPTION
Before copying this script, replace REPLACE_WITH_CLIENT_PUBLIC_KEY below with
your client's public key. Copy only this script to each server.
.EXAMPLE
.\bootstrap-windows.ps1 -AllowedSource '203.0.113.10/32'
#>
[CmdletBinding(SupportsShouldProcess = $true, DefaultParameterSetName = 'Embedded')]
param(
    # Embed the public key here on the client; never embed the private key.
    [Parameter(ParameterSetName = 'Embedded')]
    [string]$PublicKey = 'REPLACE_WITH_CLIENT_PUBLIC_KEY',
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
    param([string]$Existing, [string]$Account, [int]$Port)
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
        if ($line -match '^\s*Subsystem\s+sftp\s+') { $subsystem = $true }
        $global.Add($line)
    }
    if (!$subsystem) { $global.Add('Subsystem sftp sftp-server.exe') }
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

function Invoke-DayZSSHBootstrap {
    [CmdletBinding(SupportsShouldProcess = $true)]
    param([string]$KeyPath, [string]$Key, [string[]]$Sources, [string]$Account = 'Administrator', [int]$Port = 22)
    $ErrorActionPreference = 'Stop'
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

    $feature = Get-WindowsCapability -Online -Name 'OpenSSH.Server~~~~0.0.1.0'
    if ($feature.State -ne 'Installed') {
        $installed = Add-WindowsCapability -Online -Name 'OpenSSH.Server~~~~0.0.1.0'
        if ($installed.RestartNeeded) { throw 'Windows reports that OpenSSH installation requires a reboot. Rerun this script after Windows finishes servicing.' }
    }
    $sshDir = Join-Path $env:WINDIR 'System32\OpenSSH'
    $sshd = Join-Path $sshDir 'sshd.exe'
    $keygen = Join-Path $sshDir 'ssh-keygen.exe'
    $root = Join-Path $env:ProgramData 'ssh'
    New-Item -ItemType Directory -Force -Path $root | Out-Null
    Invoke-DeployNative $keygen @('-A') | Out-Null
    Get-ChildItem -LiteralPath $root -Filter 'ssh_host_*_key' -File | ForEach-Object { Set-DeployPrivateAcl $_.FullName }
    $cfgPath = Join-Path $root 'sshd_config'
    $keysPath = Join-Path $root 'administrators_authorized_keys'
    $cfgExisted = Test-Path -LiteralPath $cfgPath
    $keysExisted = Test-Path -LiteralPath $keysPath
    $oldCfg = [IO.File]::ReadAllText((Join-Path $sshDir 'sshd_config_default'))
    $oldKeys = ''
    $oldCfgAcl = $null
    $oldKeysAcl = $null
    if ($cfgExisted) { $oldCfg = [IO.File]::ReadAllText($cfgPath); $oldCfgAcl = Get-Acl -LiteralPath $cfgPath }
    if ($keysExisted) { $oldKeys = [IO.File]::ReadAllText($keysPath); $oldKeysAcl = Get-Acl -LiteralPath $keysPath }
    $newCfg = New-DeploySshConfig $oldCfg $Account $Port
    $newKeys = Merge-DeployAuthorizedKeys $oldKeys $key
    $utf8 = New-Object Text.UTF8Encoding($false)
    $backup = Join-Path $root ('dzdeploy-backup-' + [DateTime]::UtcNow.ToString('yyyyMMddTHHmmssfffffff'))
    New-Item -ItemType Directory -Path $backup | Out-Null
    Set-DeployPrivateAcl $backup -Directory
    # Validate the embedded key using OpenSSH without requiring a second input file.
    $keyCandidate = Join-Path $backup 'client-public-key.pub'
    [IO.File]::WriteAllText($keyCandidate, $key, $utf8)
    Invoke-DeployNative $keygen @('-lf', $keyCandidate, '-E', 'sha256') | Out-Null
    [IO.File]::WriteAllText((Join-Path $backup 'sshd_config'), $oldCfg, $utf8)
    [IO.File]::WriteAllText((Join-Path $backup 'administrators_authorized_keys'), $oldKeys, $utf8)
    $service = Get-Service sshd
    $oldRunning = $service.Status -eq 'Running'
    $oldStart = $service.StartType
    $ruleName = 'DayZDeploy-SSH'
    $oldRule = Get-NetFirewallRule -Name $ruleName -ErrorAction SilentlyContinue
    $oldPort = $null; $oldAddress = $null; $oldApplication = $null
    if ($oldRule) {
        $oldPort = $oldRule | Get-NetFirewallPortFilter
        $oldAddress = $oldRule | Get-NetFirewallAddressFilter
        $oldApplication = $oldRule | Get-NetFirewallApplicationFilter
    }
    $broad = @(Get-NetFirewallRule -Name 'OpenSSH-Server-In-TCP' -ErrorAction SilentlyContinue)
    try {
        # The new files inherit a private DACL before any key/config bytes land.
        foreach ($path in @($cfgPath, $keysPath)) {
            if (!(Test-Path -LiteralPath $path)) { [IO.File]::WriteAllBytes($path, [byte[]]@()) }
            Set-DeployPrivateAcl $path
        }
        $candidate = Join-Path $backup 'sshd_config.candidate'
        [IO.File]::WriteAllText($candidate, $newCfg, $utf8)
        Invoke-DeployNative $sshd @('-t', '-f', $candidate) | Out-Null
        [IO.File]::WriteAllText($keysPath, $newKeys, $utf8)
        [IO.File]::WriteAllText($cfgPath, $newCfg, $utf8)
        Invoke-DeployNative $sshd @('-t', '-f', $cfgPath) | Out-Null
        # Create the scoped allow rule before disabling the default broad rule.
        if ($oldRule) { Remove-NetFirewallRule -Name $ruleName }
        New-NetFirewallRule -Name $ruleName -DisplayName 'DayZ Deploy SSH' -Direction Inbound -Action Allow -Protocol TCP -LocalPort $Port -RemoteAddress $Sources -Program $sshd -Profile Any | Out-Null
        foreach ($rule in $broad) { Disable-NetFirewallRule -Name $rule.Name | Out-Null }
        Set-Service sshd -StartupType Automatic
        if ($oldRunning) { Restart-Service sshd } else { Start-Service sshd }
        (Get-Service sshd).WaitForStatus('Running', [TimeSpan]::FromSeconds(30))
        $fingerprint = Invoke-DeployNative $keygen @('-lf', (Join-Path $root 'ssh_host_ed25519_key.pub'), '-E', 'sha256')
        Write-Host "SSH ready: user=$Account port=$Port sources=$($Sources -join ',')"
        Write-Host "Trusted host fingerprint: $fingerprint"
        Write-Host "Next, on the client: dzdeploy deploy <target> --restart --host-key <SHA256:fingerprint-above>"
        Write-Host "SSH backup: $backup"
    } catch {
        $failure = $_
        # Restore original configuration, keys, permissions, firewall and service.
        try {
            if ($cfgExisted) { [IO.File]::WriteAllText($cfgPath, $oldCfg, $utf8); Set-Acl -LiteralPath $cfgPath -AclObject $oldCfgAcl } else { Remove-Item -LiteralPath $cfgPath -Force -ErrorAction SilentlyContinue }
            if ($keysExisted) { [IO.File]::WriteAllText($keysPath, $oldKeys, $utf8); Set-Acl -LiteralPath $keysPath -AclObject $oldKeysAcl } else { Remove-Item -LiteralPath $keysPath -Force -ErrorAction SilentlyContinue }
            Get-NetFirewallRule -Name $ruleName -ErrorAction SilentlyContinue | Remove-NetFirewallRule
            if ($oldRule) { New-NetFirewallRule -Name $ruleName -DisplayName $oldRule.DisplayName -Enabled $oldRule.Enabled -Direction $oldRule.Direction -Action $oldRule.Action -Protocol $oldPort.Protocol -LocalPort $oldPort.LocalPort -RemoteAddress $oldAddress.RemoteAddress -Program $oldApplication.Program -Profile $oldRule.Profile | Out-Null }
            foreach ($rule in $broad) { Set-NetFirewallRule -Name $rule.Name -Enabled $rule.Enabled | Out-Null }
            Set-Service sshd -StartupType $oldStart
            if ($oldRunning) { Restart-Service sshd } else { Stop-Service sshd -ErrorAction SilentlyContinue }
        } catch { Write-Warning "SSH rollback needs attention: $_. Backup: $backup" }
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
