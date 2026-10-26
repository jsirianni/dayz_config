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
$script:calls = @{}
$script:aclCapture = $null
function Count-Call([string]$Name) { if (!$script:calls.ContainsKey($Name)) { $script:calls[$Name] = 0 }; $script:calls[$Name]++ }
function Assert-DeployAdministrator { Count-Call 'admin' }
function Get-LocalUser { param($Name); Count-Call 'account'; return [pscustomobject]@{ Enabled = $script:userEnabled; SID = [pscustomobject]@{ Value = 'S-1-5-21-1-2-3-500' } } }
function Get-LocalGroupMember { param($SID); return [pscustomobject]@{ SID = [pscustomobject]@{ Value = 'S-1-5-21-1-2-3-500' } } }
function Write-MockCapabilityPayload([string]$Name) {
    $files = @('ssh-keygen.exe')
    if ($Name -eq $script:serverCapability) { $files = @('sshd.exe', 'sftp-server.exe', 'sshd_config_default') }
    foreach ($file in $files) {
        $content = 'mock executable; never execute'
        if ($file -eq 'sshd_config_default') { $content = $stock }
        [IO.File]::WriteAllText((Join-Path $sshDir $file), $content, $utf8)
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
    foreach ($file in @('ssh-keygen.exe', 'sshd.exe', 'sftp-server.exe', 'sshd_config_default')) {
        $path = Join-Path $sshDir $file
        if (Test-Path -LiteralPath $path) { Remove-Item -LiteralPath $path }
    }
    if ($Client -eq 'Installed') { Write-MockCapabilityPayload $script:clientCapability }
    if ($Server -eq 'Installed') { Write-MockCapabilityPayload $script:serverCapability }
}
function Assert-NoSSHMutation {
    foreach ($call in @('native', 'service-query', 'set-service', 'restart', 'start', 'firewall', 'acl')) {
        Assert-Equal $script:calls.ContainsKey($call) $false "Prerequisite failure reached $call"
    }
}
function Set-Acl { param($LiteralPath, $AclObject); $script:aclCapture = $AclObject; Count-Call 'acl' }
function Get-Acl { param($LiteralPath); return (New-Object Security.AccessControl.FileSecurity) }
function Invoke-DeployNative {
    param($Executable, [string[]]$ArgList)
    Count-Call 'native'
    if (!(Test-Path -LiteralPath $Executable -PathType Leaf)) { throw "Native executable is missing: $Executable" }
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
function Set-Service { param($Name, $StartupType); $script:lastStartup = $StartupType; Count-Call 'set-service' }
function Restart-Service { param($Name); Count-Call 'restart'; if ($script:failRestart) { $script:failRestart = $false; throw 'mock startup failure' } }
function Start-Service { param($Name); Count-Call 'start' }
function Stop-Service { [CmdletBinding()]param($Name); Count-Call 'stop' }
function Get-NetFirewallRule {
    [CmdletBinding()]param($Name)
    if ($Name -eq 'OpenSSH-Server-In-TCP') { return [pscustomobject]@{ Name = $Name; Enabled = 'True' } }
    return $script:managedRule
}
function Get-NetFirewallPortFilter { param([Parameter(ValueFromPipeline)]$InputObject); process { return [pscustomobject]@{Protocol=$InputObject.Protocol;LocalPort=$InputObject.LocalPort} } }
function Get-NetFirewallAddressFilter { param([Parameter(ValueFromPipeline)]$InputObject); process { return [pscustomobject]@{RemoteAddress=$InputObject.RemoteAddress} } }
function Get-NetFirewallApplicationFilter { param([Parameter(ValueFromPipeline)]$InputObject); process { return [pscustomobject]@{Program=$InputObject.Program} } }
function New-NetFirewallRule {
    param($Name, $DisplayName, $Direction, $Action, $Protocol, $LocalPort, $RemoteAddress, $Program, $Profile, $Enabled)
    $script:lastRule = @{} + $PSBoundParameters
    $script:managedRule = [pscustomobject]@{Name=$Name;DisplayName=$DisplayName;Direction=$Direction;Action=$Action;Protocol=$Protocol;LocalPort=$LocalPort;RemoteAddress=$RemoteAddress;Program=$Program;Profile=$Profile;Enabled='True'}
    Count-Call 'firewall'
}
function Disable-NetFirewallRule { param($Name); Count-Call 'disable-broad' }
function Set-NetFirewallRule { param($Name, $Enabled); Count-Call 'restore-broad' }
function Remove-NetFirewallRule { [CmdletBinding()]param($Name, [Parameter(ValueFromPipeline)]$InputObject); process { Count-Call 'remove-rule'; $script:managedRule=$null } }

try {
    New-Item -ItemType Directory -Path $taskRoot | Out-Null
    $env:WINDIR = Join-Path $taskRoot 'Windows'
    $env:ProgramData = Join-Path $taskRoot 'ProgramData'
    $sshDir = Join-Path $env:WINDIR 'System32\OpenSSH'
    $stateDir = Join-Path $env:ProgramData 'ssh'
    New-Item -ItemType Directory -Force -Path $sshDir, $stateDir | Out-Null
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

    # Regression: Server installed with Client/keygen absent.
    Reset-MockOpenSSH -Client 'NotPresent'
    Assert-Equal (Test-Path -LiteralPath (Join-Path $sshDir 'ssh-keygen.exe')) $false 'Missing-client setup still had keygen'
    Assert-Equal (Install-DeployOpenSSH) $sshDir 'Missing client was not installed'
    Assert-Equal $script:calls["install-$script:clientCapability"] 1 'Client capability was not installed exactly once'
    Assert-Equal $script:calls.ContainsKey("install-$script:serverCapability") $false 'Existing server capability was reinstalled'
    Assert-Equal $script:calls["query-$script:clientCapability"] 2 'Client install state was not verified after installation'
    Assert-Equal (Test-Path -LiteralPath (Join-Path $sshDir 'ssh-keygen.exe')) $true 'Client installation did not supply keygen'

    Reset-MockOpenSSH -Client 'NotPresent' -Server 'NotPresent'
    Assert-Equal (Install-DeployOpenSSH) $sshDir 'Fresh host prerequisites failed'
    Assert-Equal $script:calls.install 2 'Fresh host did not install both capabilities'
    $script:calls = @{}
    Assert-Equal (Install-DeployOpenSSH) $sshDir 'Installed host prerequisites failed'
    Assert-Equal $script:calls.ContainsKey('install') $false 'Healthy capabilities were reinstalled'

    Reset-MockOpenSSH -Client 'NotPresent'
    $script:postInstallState = 'NotPresent'
    Assert-ThrowsContaining { Invoke-DayZSSHBootstrap -Key $key -Sources '203.0.113.10/32' } @($script:clientCapability, 'NotPresent', 'DISM\dism.log')
    Assert-NoSSHMutation

    foreach ($file in @('ssh-keygen.exe', 'sshd.exe', 'sftp-server.exe', 'sshd_config_default')) {
        Reset-MockOpenSSH
        Remove-Item -LiteralPath (Join-Path $sshDir $file)
        Assert-ThrowsContaining { Invoke-DayZSSHBootstrap -Key $key -Sources '203.0.113.10/32' } @('installation is incomplete', (Join-Path $sshDir $file), 'Installed', 'DISM\dism.log')
        Assert-NoSSHMutation
    }
    Reset-MockOpenSSH -Client 'NotPresent'
    $script:skipPayload = $true
    Assert-ThrowsContaining { Invoke-DayZSSHBootstrap -Key $key -Sources '203.0.113.10/32' } @('installation is incomplete', 'ssh-keygen.exe')
    Assert-NoSSHMutation

    Reset-MockOpenSSH -Client 'NotPresent'
    $script:restartNeeded = $true
    Assert-ThrowsContaining { Invoke-DayZSSHBootstrap -Key $key -Sources '203.0.113.10/32' } @($script:clientCapability, 'requires a reboot')
    Assert-NoSSHMutation
    Reset-MockOpenSSH -Client 'InstallPending'
    Assert-ThrowsContaining { Invoke-DayZSSHBootstrap -Key $key -Sources '203.0.113.10/32' } @('InstallPending', 'Reboot')
    Assert-Equal $script:calls.ContainsKey('install') $false 'Pending capability was reinstalled'
    Assert-NoSSHMutation
    Reset-MockOpenSSH -Client 'NotPresent'
    $script:postInstallState = 'InstallPending'
    Assert-ThrowsContaining { Invoke-DayZSSHBootstrap -Key $key -Sources '203.0.113.10/32' } @('InstallPending', 'Reboot')
    Assert-NoSSHMutation

    Reset-MockOpenSSH -Server 'NotPresent'
    $script:failInstall = $script:serverCapability
    Assert-ThrowsContaining { Invoke-DayZSSHBootstrap -Key $key -Sources '203.0.113.10/32' } @($script:serverCapability, '0x800f0954', 'DISM\dism.log')
    Assert-NoSSHMutation
    Reset-MockOpenSSH
    $script:failQuery = $script:clientCapability
    Assert-ThrowsContaining { Invoke-DayZSSHBootstrap -Key $key -Sources '203.0.113.10/32' } @($script:clientCapability, '0x800f081f', 'DISM\dism.log')
    Assert-NoSSHMutation
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
    Remove-Item -LiteralPath $pub
    Invoke-DayZSSHBootstrap -Key "$key embedded-client" -Sources '203.0.113.10/32' -Port 2222
    Assert-Equal $script:calls["install-$script:clientCapability"] 1 'Bootstrap did not install the missing client tools before invoking keygen'
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

    foreach ($failure in @('failValidate', 'failRestart')) {
        Set-Variable -Scope Script -Name $failure -Value $true
        Assert-Throws { Invoke-DayZSSHBootstrap -KeyPath $pub -Sources '203.0.113.20/32' -Port 2022 } 'Injected failure was ignored'
        Assert-Equal ([IO.File]::ReadAllText($cfgPath)) $beforeCfg 'Rollback did not restore SSH config'
        Assert-Equal ([IO.File]::ReadAllText($keysPath)) $beforeKeys 'Rollback did not restore authorized keys'
        Assert-Equal $script:lastStartup 'Manual' 'Rollback did not restore startup mode'
        Assert-Equal $script:managedRule.LocalPort 2222 'Rollback did not restore the previous scoped firewall rule'
        Assert-Equal ($script:managedRule.RemoteAddress -join ',') '203.0.113.10/32' 'Rollback changed the previous allowed network'
        Set-Variable -Scope Script -Name $failure -Value $false
    }
    Write-Host 'All mocked Windows SSH bootstrap tests passed.'
} finally {
    $env:WINDIR = $savedWindows
    $env:ProgramData = $savedData
    # Delete only the absolute, known test directory created above.
    $resolved = [IO.Path]::GetFullPath($taskRoot)
    $tempPrefix = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd('\') + '\'
    if (!$resolved.StartsWith($tempPrefix, [StringComparison]::OrdinalIgnoreCase) -or [IO.Path]::GetFileName($resolved) -notlike 'dzdeploy-ssh-test-*') { throw 'Unsafe test cleanup path' }
    if (Test-Path -LiteralPath $resolved) { Remove-Item -LiteralPath $resolved -Recurse -Force }
}
