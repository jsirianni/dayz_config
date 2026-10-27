#Requires -Version 5.1
[CmdletBinding(SupportsShouldProcess = $true)]
param(
    [Parameter(Mandatory = $true)][ValidatePattern('^[A-Za-z0-9][A-Za-z0-9.:-]*$')][string]$SSHHost,
    [Parameter(Mandatory = $true)][ValidatePattern('^[A-Za-z_][A-Za-z0-9_-]*$')][string]$User,
    [ValidateRange(1, 65535)][int]$Port = 22,
    [string]$IdentityFile = (Join-Path $HOME '.ssh/id_ed25519'),
    [string]$KnownHostsFile = (Join-Path $HOME '.ssh/known_hosts'),
    [string]$ConfigPath = (Join-Path $PSScriptRoot '../../.deploy/dzsa-notifier.yaml')
)
$ErrorActionPreference = 'Stop'
$ssh = (Get-Command ssh -CommandType Application -ErrorAction Stop).Source
$key = (Resolve-Path -LiteralPath $IdentityFile).ProviderPath
$config = (Resolve-Path -LiteralPath $ConfigPath).ProviderPath
$bytes = [IO.File]::ReadAllBytes($config)
if ($bytes.Length -gt 1048576) { throw 'Configuration exceeds 1 MiB' }
$hasher = [Security.Cryptography.SHA256]::Create()
try { $hash = ([BitConverter]::ToString($hasher.ComputeHash($bytes))).Replace('-', '').ToLowerInvariant() }
finally { $hasher.Dispose() }
$mode = 'apply'
if ($WhatIfPreference) { $mode = 'preview' }
elseif (!$PSCmdlet.ShouldProcess("${User}@${SSHHost}:$Port", 'Apply authoritative dzsa-notifier configuration and activate service')) { return }
$trust = 'accept-new'
if ($mode -eq 'preview') { $trust = 'yes' }
$arguments = @('-T', '-p', "$Port", '-i', $key, '-o', 'IdentitiesOnly=yes', '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=15', '-o', 'ServerAliveInterval=10', '-o', 'ServerAliveCountMax=3', '-o', 'UpdateHostKeys=no', '-o', 'CheckHostIP=no', '-o', "StrictHostKeyChecking=$trust", '-o', "UserKnownHostsFile=$([IO.Path]::GetFullPath($KnownHostsFile))", "${User}@${SSHHost}", 'if [ "$(id -u)" -eq 0 ]; then sh -s; else sudo -n sh -s; fi')
$helper = [IO.File]::ReadAllText((Join-Path $PSScriptRoot 'apply-config.sh')).Replace("`r`n", "`n")
$payload = "MODE='$mode'`nEXPECTED_SHA='$hash'`nPAYLOAD='$([Convert]::ToBase64String($bytes))'`n" + $helper
# ProcessStartInfo avoids PowerShell's pipeline encoding and CRLF conversion.
$start = New-Object Diagnostics.ProcessStartInfo
$start.FileName = $ssh
$start.UseShellExecute = $false
$start.RedirectStandardInput = $true
if ($start.PSObject.Properties.Name -contains 'ArgumentList') {
    foreach ($argument in $arguments) { $start.ArgumentList.Add($argument) }
} else {
    # Windows command-line quoting for Windows PowerShell 5.1.
    $quoted = foreach ($argument in $arguments) { '"' + (($argument -replace '(\\*)"', '$1$1\"') -replace '(\\+)$', '$1$1') + '"' }
    $start.Arguments = $quoted -join ' '
}
$process = New-Object Diagnostics.Process
$process.StartInfo = $start
try {
    [void]$process.Start()
    $process.StandardInput.Write($payload)
    $process.StandardInput.Close()
    $process.WaitForExit()
    if ($process.ExitCode -ne 0) { throw "SSH configuration failed (exit $($process.ExitCode)); inspect the output above. Changed host keys are never accepted." }
} finally { $process.Dispose() }
