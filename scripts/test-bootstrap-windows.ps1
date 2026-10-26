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

$taskRoot = Join-Path ([IO.Path]::GetTempPath()) ('dzdeploy-ssh-test-' + [Guid]::NewGuid().ToString('N'))
$savedWindows = $env:WINDIR
$savedData = $env:ProgramData
$script:calls = @{}
$script:aclCapture = $null
function Count-Call([string]$Name) { if (!$script:calls.ContainsKey($Name)) { $script:calls[$Name] = 0 }; $script:calls[$Name]++ }
function Assert-DeployAdministrator { Count-Call 'admin' }
function Get-LocalUser { param($Name); return [pscustomobject]@{ Enabled = $script:userEnabled; SID = [pscustomobject]@{ Value = 'S-1-5-21-1-2-3-500' } } }
function Get-LocalGroupMember { param($SID); return [pscustomobject]@{ SID = [pscustomobject]@{ Value = 'S-1-5-21-1-2-3-500' } } }
function Get-WindowsCapability { param([switch]$Online, $Name); Count-Call 'capability'; return [pscustomobject]@{ State = 'NotPresent' } }
function Add-WindowsCapability { param([switch]$Online, $Name); Count-Call 'install'; return [pscustomobject]@{ RestartNeeded = $false } }
function Set-Acl { param($LiteralPath, $AclObject); $script:aclCapture = $AclObject; Count-Call 'acl' }
function Get-Acl { param($LiteralPath); return (New-Object Security.AccessControl.FileSecurity) }
function Invoke-DeployNative {
    param($Executable, [string[]]$ArgList)
    Count-Call 'native'
    if ($ArgList[0] -eq '-t' -and $script:failValidate) { throw 'mock sshd configuration failure' }
    if ($ArgList[0] -eq '-lf') { return '256 SHA256:trusted-test-key host (ED25519)' }
}
function Get-Service {
    param($Name)
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
    Invoke-DayZSSHBootstrap -Key "$key embedded-client" -Sources '203.0.113.10/32' -WhatIf
    Assert-Equal $script:calls.ContainsKey('native') $false 'Embedded-key WhatIf ran native commands'
    Assert-Throws { Invoke-DayZSSHBootstrap -Key $key -KeyPath $pub -Sources '203.0.113.10/32' } 'Ambiguous key inputs accepted'
    Assert-Throws { Invoke-DayZSSHBootstrap -Key 'missing-key' -Sources '203.0.113.10/32' } 'Unconfigured embedded key reached setup'
    Assert-Equal $script:calls.ContainsKey('capability') $false 'Invalid key changed Windows capabilities'
    Assert-Throws { Invoke-DayZSSHBootstrap -KeyPath $pub -Sources '0.0.0.0/0' } 'Unrestricted source accepted'
    $script:userEnabled = $false
    Assert-Throws { Invoke-DayZSSHBootstrap -KeyPath $pub -Sources '203.0.113.10/32' } 'Disabled account accepted'
    $script:userEnabled = $true

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
    Remove-Item -LiteralPath $pub
    Invoke-DayZSSHBootstrap -Key "$key embedded-client" -Sources '203.0.113.10/32' -Port 2222
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
