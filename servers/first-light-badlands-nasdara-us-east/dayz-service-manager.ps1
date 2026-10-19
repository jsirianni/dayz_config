#Requires -Version 5.1
<#
.SYNOPSIS
Runs the Badlands update/launch cycle again after each server exit.
.DESCRIPTION
Console supervisor, matching the Deer Isle manager; not a Windows service installer.
Press Ctrl+C to stop the manager. Do not run multiple managers for one installation.
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateNotNullOrEmpty()]
    [string]$SteamUsername,

    [ValidateNotNullOrEmpty()]
    [string]$SteamCmdPath = 'C:\SteamCMD\steamcmd.exe',

    [ValidateNotNullOrEmpty()]
    [string]$InstallDirectory = 'C:\DayZ\server',

    [string]$WorkshopDirectory,

    [ValidateRange(1, 3600)]
    [int]$RestartDelaySeconds = 10
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$serviceScript = Join-Path $PSScriptRoot 'dayz-service.ps1'
$serviceParameters = @{
    SteamUsername = $SteamUsername
    SteamCmdPath = $SteamCmdPath
    InstallDirectory = $InstallDirectory
}
if (-not [string]::IsNullOrWhiteSpace($WorkshopDirectory)) {
    $serviceParameters.WorkshopDirectory = $WorkshopDirectory
}

Write-Host 'Badlands manager starting. Press Ctrl+C to stop.'
while ($true) {
    Write-Host "[$(Get-Date -Format s)] Starting update/launch cycle..."
    & $serviceScript @serviceParameters
    $cycleExitCode = $LASTEXITCODE
    Write-Host "[$(Get-Date -Format s)] Cycle exited with code $cycleExitCode. Restarting in $RestartDelaySeconds seconds..."
    Start-Sleep -Seconds $RestartDelaySeconds
}
