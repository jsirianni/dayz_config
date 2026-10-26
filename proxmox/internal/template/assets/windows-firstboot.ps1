$ErrorActionPreference = 'Stop'
# Cloudbase-Init creates the user/profile and writes its metadata SSH keys first.
$source = 'C:\Users\pveadmin\.ssh\authorized_keys'
if (-not (Test-Path -LiteralPath $source)) { throw 'Cloudbase-Init public keys are missing' }
$keys = Get-Content -LiteralPath $source | Where-Object { $_.Trim() }
if (-not $keys) { throw 'Cloudbase-Init provided no public keys' }
$target = Join-Path $env:ProgramData 'ssh\administrators_authorized_keys'
$keys | Set-Content -LiteralPath $target -Encoding ascii
& icacls.exe $target /inheritance:r /grant '*S-1-5-18:F' '*S-1-5-32-544:F' | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'Cannot protect administrator keys' }
& "$env:WINDIR\System32\OpenSSH\ssh-keygen.exe" -A
if ($LASTEXITCODE -ne 0) { throw 'Cannot generate SSH host keys' }
Restart-Service sshd
