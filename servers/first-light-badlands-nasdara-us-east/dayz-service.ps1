#Requires -Version 5.1
<#
.SYNOPSIS
Updates DayZ, CF and VPPAdminTools, then runs the Badlands server once.
.DESCRIPTION
Use -UpdateOnly for installation before deploying the repository configuration.
SteamCMD handles password and Steam Guard prompts interactively.
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

    [switch]$UpdateOnly
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
# Robocopy uses success codes 0-7, including on PowerShell 7 hosts.
$PSNativeCommandUseErrorActionPreference = $false

try {
    if ($env:OS -ne 'Windows_NT') {
        throw 'This launcher requires Windows.'
    }

    $SteamCmdPath = (Get-Item -LiteralPath $SteamCmdPath -ErrorAction Stop).FullName
    if (-not [System.IO.Path]::IsPathRooted($InstallDirectory)) {
        throw 'InstallDirectory must be an absolute Windows path.'
    }
    $installRoot = [System.IO.Path]::GetFullPath($InstallDirectory).TrimEnd('\')
    if ([string]::IsNullOrWhiteSpace($WorkshopDirectory)) {
        $WorkshopDirectory = Join-Path (Split-Path -Parent $SteamCmdPath) 'steamapps\workshop\content\221100'
    }
    if (-not [System.IO.Path]::IsPathRooted($WorkshopDirectory)) {
        throw 'WorkshopDirectory must be an absolute Windows path.'
    }
    $workshopRoot = [System.IO.Path]::GetFullPath($WorkshopDirectory).TrimEnd('\')
    $profilesDirectory = Join-Path $installRoot 'profiles'
    $keysDirectory = Join-Path $installRoot 'keys'
    $serverExe = Join-Path $installRoot 'DayZServer_x64.exe'
    $mods = @(
        @{ Id = '1559212036'; Folder = '@CF' }
        @{ Id = '1828439124'; Folder = '@VPPAdminTools' }
    )

    foreach ($directory in @($installRoot, $profilesDirectory, $keysDirectory)) {
        New-Item -ItemType Directory -Path $directory -Force | Out-Null
    }

    Write-Host 'Updating DayZ server (223350)...'
    & $SteamCmdPath '+force_install_dir' $installRoot '+login' $SteamUsername '+app_update' '223350' '+quit'
    if ($LASTEXITCODE -ne 0) {
        throw "SteamCMD server update failed with exit code $LASTEXITCODE."
    }
    if (-not (Test-Path -LiteralPath $serverExe -PathType Leaf)) {
        throw "Missing DayZ server executable: $serverExe"
    }

    # A separate Workshop invocation keeps downloads under the SteamCMD directory,
    # matching the Deer Isle Windows launchers.
    Write-Host 'Updating CF and VPPAdminTools (Workshop app 221100)...'
    $workshopArguments = @('+login', $SteamUsername)
    foreach ($mod in $mods) {
        $workshopArguments += @('+workshop_download_item', '221100', $mod.Id)
    }
    $workshopArguments += '+quit'
    & $SteamCmdPath @workshopArguments
    if ($LASTEXITCODE -ne 0) {
        throw "SteamCMD Workshop update failed with exit code $LASTEXITCODE."
    }

    foreach ($mod in $mods) {
        $source = Join-Path $workshopRoot $mod.Id
        $destination = [System.IO.Path]::GetFullPath((Join-Path $installRoot $mod.Folder))
        if (-not (Test-Path -LiteralPath $source -PathType Container)) {
            throw "Missing Workshop mod: $source"
        }
        # /MIR may delete stale files: restrict it to the dedicated mod folder.
        if ((Split-Path -Parent $destination) -ne $installRoot -or
            $source.StartsWith($destination + '\', [System.StringComparison]::OrdinalIgnoreCase) -or
            $destination.StartsWith($source + '\', [System.StringComparison]::OrdinalIgnoreCase) -or
            $source -eq $destination) {
            throw "Unsafe mod mirror destination: $destination"
        }
        if ((Test-Path -LiteralPath $destination) -and
            ((Get-Item -LiteralPath $destination -Force).Attributes -band [System.IO.FileAttributes]::ReparsePoint)) {
            throw "Mod destination must be a real directory, not a link: $destination"
        }

        # Windows resolves both keys/Keys; also accept keys at the mod root.
        $signingKeys = @(Get-ChildItem -LiteralPath $source -Filter '*.bikey' -File)
        $modKeysDirectory = Join-Path $source 'keys'
        if (Test-Path -LiteralPath $modKeysDirectory -PathType Container) {
            $signingKeys += @(Get-ChildItem -LiteralPath $modKeysDirectory -Filter '*.bikey' -File)
        }
        if ($signingKeys.Count -eq 0) {
            throw "Missing signing keys for Workshop mod $($mod.Id)."
        }

        Write-Host "Synchronizing $($mod.Folder)..."
        & robocopy.exe $source $destination /MIR /XJ /R:2 /W:2 /NFL /NDL /NJH /NJS /NP
        if ($LASTEXITCODE -lt 0 -or $LASTEXITCODE -ge 8) {
            throw "Robocopy failed for $($mod.Folder) with exit code $LASTEXITCODE."
        }
        foreach ($key in $signingKeys) {
            Copy-Item -LiteralPath $key.FullName -Destination $keysDirectory -Force
        }
    }

    if ($UpdateOnly) {
        Write-Host 'Update complete. Deploy configuration, the Nasdara mission and VPP profiles before starting.'
        exit 0
    }

    foreach ($relativePath in @(
        'serverDZ.cfg'
        'mpmissions\dayzOffline.nasdara\init.c'
        'profiles\VPPAdminTools\Permissions\SuperAdmins\SuperAdmins.txt'
        'profiles\VPPAdminTools\Permissions\credentials.txt'
    )) {
        if (-not (Test-Path -LiteralPath (Join-Path $installRoot $relativePath) -PathType Leaf)) {
            throw "Deploy the repository configuration before starting; missing $relativePath."
        }
    }

    $serverArguments = @(
        '-config=serverDZ.cfg'
        '-port=2301'
        '-mod=@CF;@VPPAdminTools'
        '-BEpath=battleye'
        '-profiles=profiles'
        '-dologs'
        '-adminlog'
        '-netlog'
        '-freezecheck'
    )
    Write-Host 'Starting Badlands on port 2301 with CF;VPPAdminTools...'
    Push-Location -LiteralPath $installRoot
    try {
        & $serverExe @serverArguments
        $serverExitCode = $LASTEXITCODE
    }
    finally {
        Pop-Location
    }
    Write-Host "DayZ exited with code $serverExitCode."
    exit $serverExitCode
}
catch {
    Write-Error -Message $_ -ErrorAction Continue
    exit 1
}
