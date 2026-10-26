$ErrorActionPreference = 'Stop'
function Check-Exit($Code, $Step) { if ($Code -notin @(0,3010)) { throw "$Step failed with exit $Code" } }
$seed = Split-Path -Parent $PSCommandPath
$virtio = Get-Volume | Where-Object { $_.DriveLetter -and (Test-Path "$($_.DriveLetter):\guest-agent\qemu-ga-x86_64.msi") } | Select-Object -First 1
if (-not $virtio) { throw 'VirtIO driver/guest-agent ISO not found' }
$driverVersion = '2k22'
if ([Environment]::OSVersion.Version.Build -ge 26100) { $driverVersion = '2k25' }
foreach ($driver in @('vioscsi','NetKVM','vioserial')) {
    $path = "$($virtio.DriveLetter):\$driver\$driverVersion\amd64\*.inf"
    if (-not (Test-Path $path)) { throw "Required $driver driver for $driverVersion is missing" }
    & pnputil.exe /add-driver $path /install | Out-Null
    Check-Exit $LASTEXITCODE "$driver driver installation"
}
$agent = "$($virtio.DriveLetter):\guest-agent\qemu-ga-x86_64.msi"
$p = Start-Process msiexec.exe -ArgumentList @('/i', $agent, '/qn', '/norestart') -Wait -PassThru -WindowStyle Hidden
Check-Exit $p.ExitCode 'QEMU guest agent installation'
Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0 | Out-Null
& "$env:WINDIR\System32\OpenSSH\ssh-keygen.exe" -A
Check-Exit $LASTEXITCODE 'SSH host-key generation'
$ssh = Join-Path $env:ProgramData 'ssh'
New-Item -ItemType Directory -Force -Path $ssh | Out-Null
Copy-Item -LiteralPath (Join-Path $seed 'BUILDKEY.PUB') -Destination (Join-Path $ssh 'administrators_authorized_keys') -Force
& icacls.exe (Join-Path $ssh 'administrators_authorized_keys') /inheritance:r /grant '*S-1-5-18:F' '*S-1-5-32-544:F' | Out-Null
Check-Exit $LASTEXITCODE 'authorized-key ACL'
$cfg = Join-Path $ssh 'sshd_config'
if (-not (Test-Path $cfg)) { Copy-Item "$env:WINDIR\System32\OpenSSH\sshd_config_default" $cfg }
Add-Content -LiteralPath $cfg -Value "`nMatch all`nPasswordAuthentication no`nPubkeyAuthentication yes`n"
if (-not (Get-NetFirewallRule -Name 'pvecli-ssh' -ErrorAction SilentlyContinue)) { New-NetFirewallRule -Name 'pvecli-ssh' -DisplayName 'pvecli SSH' -Direction Inbound -Action Allow -Protocol TCP -LocalPort 22 | Out-Null }
Set-Service sshd -StartupType Automatic
Start-Service sshd
Set-Service 'QEMU-GA' -StartupType Automatic
Start-Service 'QEMU-GA'
