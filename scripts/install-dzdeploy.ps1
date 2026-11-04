# Windows PowerShell 5.1 or newer. Install the complete, pinned release package.
$ErrorActionPreference = 'Stop'
$version = '0.1.0'
$checksumsSha256 = 'ecf38581f7839f7dfa5bfcd28ecdf50b6cc65adbbaf55e70a53f600516bfdbcd'
$root = Split-Path $PSScriptRoot -Parent
if ($env:OS -ne 'Windows_NT') { throw 'Use install-dzdeploy.sh on Linux.' }
$machine = $env:PROCESSOR_ARCHITEW6432
if (!$machine) { $machine = $env:PROCESSOR_ARCHITECTURE }
$arch = switch ($machine) { 'AMD64' { 'amd64' }; 'ARM64' { 'arm64' }; default { throw 'Only amd64 and arm64 are supported.' } }
$tools = Join-Path $root '.tools'
$stage = Join-Path $tools ('dzdeploy-install-' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $stage -Force | Out-Null
try {
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
    $base = "https://github.com/jsirianni/dzdeploy/releases/download/v$version"
    $archive = "dzdeploy_${version}_windows_${arch}.zip"
    $manifest = Join-Path $stage 'checksums.txt'
    Invoke-WebRequest -UseBasicParsing -Uri "$base/checksums.txt" -OutFile $manifest -TimeoutSec 120
    if ((Get-FileHash -LiteralPath $manifest -Algorithm SHA256).Hash -ne $checksumsSha256) { throw 'Release checksum manifest SHA-256 mismatch.' }
    $matchesForArchive = @(Get-Content -LiteralPath $manifest | Where-Object { $_ -match ('^([0-9a-f]{64})\s+' + [regex]::Escape($archive) + '$') })
    if ($matchesForArchive.Count -ne 1) { throw 'Missing or duplicate archive checksum.' }
    $expected = ($matchesForArchive[0] -split '\s+')[0]
    $zip = Join-Path $stage $archive
    Invoke-WebRequest -UseBasicParsing -Uri "$base/$archive" -OutFile $zip -TimeoutSec 120
    if ((Get-FileHash -LiteralPath $zip -Algorithm SHA256).Hash -ne $expected) { throw 'Release archive SHA-256 mismatch.' }
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $package = Join-Path $stage 'package'
    $files = @('dzdeploy.exe', 'agents/windows-amd64/dzdeploy.exe', 'bootstrap-windows.ps1', 'deploy.example.yaml', 'deployment.md', 'operator-workflows.md')
    $reader = [IO.Compression.ZipFile]::OpenRead($zip)
    try {
        foreach ($file in $files) {
            $entries = @($reader.Entries | Where-Object { $_.FullName -ceq $file })
            if ($entries.Count -ne 1 -or $entries[0].Length -le 0) { throw "Missing or duplicate archive entry: $file" }
            $out = Join-Path $package $file
            New-Item -ItemType Directory -Path (Split-Path $out -Parent) -Force | Out-Null
            [IO.Compression.ZipFileExtensions]::ExtractToFile($entries[0], $out)
        }
    } finally { $reader.Dispose() }
    foreach ($file in $files) {
        $out = Join-Path (Join-Path $tools 'bin') $file
        New-Item -ItemType Directory -Path (Split-Path $out -Parent) -Force | Out-Null
        Move-Item -LiteralPath (Join-Path $package $file) -Destination $out -Force
    }
    Write-Host "Installed dzdeploy v${version}: $(Join-Path $tools 'bin/dzdeploy.exe')"
} finally {
    $resolved = [IO.Path]::GetFullPath($stage)
    if ((Split-Path $resolved -Parent) -ne [IO.Path]::GetFullPath($tools) -or (Split-Path $resolved -Leaf) -notlike 'dzdeploy-install-*') { throw 'Unsafe installer cleanup path.' }
    Remove-Item -LiteralPath $resolved -Recurse -Force
}
