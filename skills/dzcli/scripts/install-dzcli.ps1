param(
    [string]$InstallDir,
    [switch]$Force,
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"

function Get-RepoRoot {
    $candidate = Join-Path $PSScriptRoot "..\..\.."
    return (Resolve-Path -LiteralPath $candidate).ProviderPath
}

function Get-HostTarget {
    $isWindowsHost = ($env:OS -eq "Windows_NT") -or ($PSVersionTable.PSEdition -eq "Desktop") -or ($IsWindows -eq $true)

    if ($isWindowsHost) {
        $platform = "windows"
        $suffix = ".exe"
        $rawArch = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
    } elseif ($IsLinux -eq $true) {
        $platform = "linux"
        $suffix = ""
        $rawArch = (& uname -m).Trim()
    } else {
        throw "Unsupported platform. dzcli release assets currently cover Windows and Linux."
    }

    switch -Regex ($rawArch) {
        "^(AMD64|X64|x86_64)$" { $arch = "amd64"; break }
        "^(ARM64|AARCH64|arm64|aarch64)$" { $arch = "arm64"; break }
        default { throw "Unsupported architecture: $rawArch" }
    }

    [pscustomobject]@{
        Platform = $platform
        Arch = $arch
        Suffix = $suffix
        FileName = if ($suffix -eq ".exe") { "dzcli.exe" } else { "dzcli" }
    }
}

function Get-InstalledVersion {
    param([string]$Path)

    if (-not (Test-Path -LiteralPath $Path)) {
        return $null
    }

    try {
        $output = (& $Path version 2>$null | Out-String).Trim()
    } catch {
        return "unknown"
    }

    if ($output -match "v?(\d+\.\d+\.\d+(?:[-+][0-9A-Za-z\.-]+)?)") {
        return $matches[1]
    }

    if ($output) {
        return $output
    }

    return "unknown"
}

function Convert-ToComparableVersion {
    param([string]$Version)

    if ([string]::IsNullOrWhiteSpace($Version) -or $Version -eq "unknown") {
        return $null
    }

    $stable = ($Version -replace "^v", "") -replace "[-+].*$", ""
    try {
        return [version]$stable
    } catch {
        return $null
    }
}

function Invoke-GitHubJson {
    param([string]$Uri)

    $headers = @{
        "Accept" = "application/vnd.github+json"
        "User-Agent" = "codex-dayz-config-dzcli-skill"
    }

    Invoke-RestMethod -Uri $Uri -Headers $headers
}

if ([string]::IsNullOrWhiteSpace($InstallDir)) {
    $InstallDir = Join-Path (Join-Path (Get-RepoRoot) ".tools") "dzcli"
}

$target = Get-HostTarget
$dzcliPath = Join-Path $InstallDir $target.FileName
$release = Invoke-GitHubJson "https://api.github.com/repos/jsirianni/dzcli/releases/latest"
$latestTag = [string]$release.tag_name
$latestVersion = $latestTag -replace "^v", ""

$assetPattern = "^dzcli_" + [regex]::Escape($latestVersion) + "_" + $target.Platform + "_" + $target.Arch + [regex]::Escape($target.Suffix) + "$"
$asset = @($release.assets | Where-Object { $_.name -match $assetPattern } | Select-Object -First 1)

if (-not $asset) {
    $fallbackPattern = "^dzcli_.*_" + $target.Platform + "_" + $target.Arch + [regex]::Escape($target.Suffix) + "$"
    $asset = @($release.assets | Where-Object { $_.name -match $fallbackPattern } | Select-Object -First 1)
}

if (-not $asset) {
    throw "No dzcli release asset found for $($target.Platform)_$($target.Arch) in $latestTag."
}

$installedVersion = Get-InstalledVersion $dzcliPath
$installedComparable = Convert-ToComparableVersion $installedVersion
$latestComparable = Convert-ToComparableVersion $latestVersion
$needsInstall = $Force -or (-not (Test-Path -LiteralPath $dzcliPath)) -or (-not $installedComparable) -or ($installedComparable -lt $latestComparable)

if (-not $needsInstall) {
    Write-Output "dzcli is current: v$installedVersion"
    Write-Output "Path: $dzcliPath"
    exit 0
}

if ($DryRun) {
    if ($installedVersion) {
        Write-Output "Installed dzcli: $installedVersion"
    } else {
        Write-Output "Installed dzcli: not found"
    }
    Write-Output "Latest dzcli: $latestTag"
    Write-Output "Would download: $($asset.browser_download_url)"
    Write-Output "Would write: $dzcliPath"
    exit 0
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

$tmpPath = Join-Path $InstallDir ($asset.name + ".download")
if (Test-Path -LiteralPath $tmpPath) {
    Remove-Item -LiteralPath $tmpPath -Force
}

Invoke-WebRequest -UseBasicParsing -Uri $asset.browser_download_url -OutFile $tmpPath

if ($asset.digest -and ([string]$asset.digest).StartsWith("sha256:")) {
    $expected = ([string]$asset.digest).Substring("sha256:".Length).ToLowerInvariant()
    $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $tmpPath).Hash.ToLowerInvariant()
    if ($actual -ne $expected) {
        Remove-Item -LiteralPath $tmpPath -Force
        throw "Checksum mismatch for $($asset.name). Expected $expected but got $actual."
    }
}

Move-Item -LiteralPath $tmpPath -Destination $dzcliPath -Force

if ($target.Platform -ne "windows") {
    & chmod +x $dzcliPath
}

$newVersion = Get-InstalledVersion $dzcliPath
Write-Output "Installed dzcli v$newVersion"
Write-Output "Path: $dzcliPath"
