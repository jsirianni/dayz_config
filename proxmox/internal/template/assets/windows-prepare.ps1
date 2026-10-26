$ErrorActionPreference = 'Stop'
function Check-Exit($Code, $Step) { if ($Code -notin @(0,3010)) { throw "$Step failed with exit $Code" } }
$work = 'C:\ProgramData\pvecli'
New-Item -ItemType Directory -Path $work -Force | Out-Null
$msi = Join-Path $work 'cloudbase-init.msi'
Invoke-WebRequest -UseBasicParsing -Uri '{{.CloudbaseURL}}' -OutFile $msi
if ((Get-FileHash -LiteralPath $msi -Algorithm SHA256).Hash.ToLowerInvariant() -ne '{{.CloudbaseHash}}') { throw 'Cloudbase-Init SHA-256 mismatch' }
$p = Start-Process msiexec.exe -ArgumentList @('/i', $msi, '/qn', '/norestart', 'RUN_SERVICE_AS_LOCAL_SYSTEM=1', 'USERNAME=pveadmin') -Wait -PassThru -WindowStyle Hidden
Check-Exit $p.ExitCode 'Cloudbase-Init installation'
Stop-Service cloudbase-init -ErrorAction SilentlyContinue
$base = 'C:\Program Files\Cloudbase Solutions\Cloudbase-Init'
$conf = Join-Path $base 'conf'
@'
[DEFAULT]
username=pveadmin
groups=Administrators
inject_user_password=false
first_logon_behaviour=no
metadata_services=cloudbaseinit.metadata.services.configdrive.ConfigDriveService
plugins=cloudbaseinit.plugins.windows.createuser.CreateUserPlugin,cloudbaseinit.plugins.common.sethostname.SetHostNamePlugin,cloudbaseinit.plugins.common.networkconfig.NetworkConfigPlugin,cloudbaseinit.plugins.common.sshpublickeys.SetUserSSHPublicKeysPlugin,cloudbaseinit.plugins.common.localscripts.LocalScriptsPlugin
allow_reboot=true
stop_service_on_exit=false
'@ | Set-Content (Join-Path $conf 'cloudbase-init.conf') -Encoding ascii
@'
[DEFAULT]
metadata_services=cloudbaseinit.metadata.services.configdrive.ConfigDriveService
plugins=cloudbaseinit.plugins.common.mtu.MTUPlugin,cloudbaseinit.plugins.common.sethostname.SetHostNamePlugin
allow_reboot=false
stop_service_on_exit=false
'@ | Set-Content (Join-Path $conf 'cloudbase-init-unattend.conf') -Encoding ascii
Copy-Item -LiteralPath (Join-Path $work 'firstboot.ps1') -Destination (Join-Path $base 'LocalScripts\01-pvecli.ps1') -Force
Remove-Item -LiteralPath 'HKLM:\SOFTWARE\Cloudbase Solutions\Cloudbase-Init' -Recurse -Force -ErrorAction SilentlyContinue
$winlogon = 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Winlogon'
foreach ($value in @('DefaultPassword','DefaultUserName','DefaultDomainName','AutoAdminLogon','AutoLogonCount')) { Remove-ItemProperty -LiteralPath $winlogon -Name $value -ErrorAction SilentlyContinue }
foreach ($path in @('C:\Windows\Panther\Unattend.xml','C:\Windows\Panther\unattend.xml','C:\Windows\System32\Sysprep\Panther\unattend.xml')) { Remove-Item -LiteralPath $path -Force -ErrorAction SilentlyContinue }
foreach ($directory in @('C:\Windows\Panther\Unattend','C:\Windows\Panther\UnattendGC')) {
    Get-ChildItem -LiteralPath $directory -File -ErrorAction SilentlyContinue | Remove-Item -Force
}
Get-ChildItem 'C:\ProgramData\ssh\ssh_host_*' -ErrorAction SilentlyContinue | Remove-Item -Force
Remove-Item -LiteralPath 'C:\ProgramData\ssh\administrators_authorized_keys' -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath 'C:\Users\Administrator\.ssh\authorized_keys' -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath $msi -Force
Remove-Item -LiteralPath (Join-Path $work 'firstboot.ps1') -Force
# Resolve the built-in Administrator by RID so non-English installations work.
$admin = Get-LocalUser | Where-Object { $_.SID.Value -match '-500$' }
Set-Service cloudbase-init -StartupType Automatic
# Replace the cached installer answer with Cloudbase-Init's password-free answer.
# Keep that final answer for the clone's specialize/OOBE passes.
$sysprep = Join-Path $env:WINDIR 'System32\Sysprep\sysprep.exe'
$p = Start-Process $sysprep -ArgumentList @('/generalize', '/oobe', '/quit', "/unattend:`"$conf\Unattend.xml`"") -WorkingDirectory (Split-Path -Parent $sysprep) -PassThru -Wait -WindowStyle Hidden
Check-Exit $p.ExitCode 'Sysprep'
if ($admin) { $admin | Disable-LocalUser }
'{{.Operation}}' | Set-Content -LiteralPath (Join-Path $work 'sealed') -NoNewline -Encoding ascii
Remove-Item -LiteralPath (Join-Path $work 'prepare.ps1') -Force
Write-Output 'pvecli-prepared'
