# Package committed configuration in an isolated fixture; never access hosts.
$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$base = Join-Path ([IO.Path]::GetTempPath()) ('dzdeploy-config-test-' + [Guid]::NewGuid().ToString('N'))
$repo = Join-Path $base 'config'
function Invoke-FixtureGit {
    param([string]$Directory, [string[]]$ArgList)
    & git -C $Directory @ArgList | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Fixture git failed: $($ArgList -join ' ')" }
}
New-Item -ItemType Directory -Path $repo -Force | Out-Null
try {
    $archive = Join-Path $base 'config.zip'
    Invoke-FixtureGit $root @('archive', '--format=zip', '--output', $archive, 'HEAD', '--', 'internal/deploy/targets.json', 'servers/current')
    Expand-Archive -LiteralPath $archive -DestinationPath $repo
    if (Get-ChildItem -LiteralPath $repo -Recurse -Filter '*.go') { throw 'Fixture must contain no Go source.' }
    Invoke-FixtureGit $repo @('init', '--initial-branch=main')
    Invoke-FixtureGit $repo @('config', 'user.name', 'Configuration integration test')
    Invoke-FixtureGit $repo @('config', 'user.email', 'config-test@example.invalid')
    Invoke-FixtureGit $repo @('config', 'core.autocrlf', 'false')
    Set-Content -LiteralPath (Join-Path $repo '.gitignore') -Value '.tools/' -Encoding Ascii
    Invoke-FixtureGit $repo @('add', '.')
    Invoke-FixtureGit $repo @('commit', '-m', 'Configuration fixture')
    $origin = Join-Path $base 'origin.git'
    Invoke-FixtureGit $base @('clone', '--bare', $repo, $origin)
    Invoke-FixtureGit $repo @('remote', 'add', 'origin', $origin)
    New-Item -ItemType Directory -Path (Join-Path $repo '.tools') | Out-Null
    Copy-Item -LiteralPath (Join-Path $root '.tools/dzcli') -Destination (Join-Path $repo '.tools') -Recurse
    foreach ($alias in @('nasdara', 'nasdara-staging', 'deer-isle')) {
        $missing = Join-Path $repo 'intentionally-missing-client.yaml'
        $stderr = Join-Path $base 'stderr.txt'
        # Missing client YAML guarantees failure before any SSH connection.
        # Windows PowerShell treats native stderr as error records. This command
        # intentionally fails; inspect its exit code and stderr explicitly.
        $ErrorActionPreference = 'Continue'
        try {
            & (Join-Path $root '.tools/bin/dzdeploy.exe') deploy $alias --repo $repo --dry-run --config $missing 2> $stderr
            $code = $LASTEXITCODE
        } finally { $ErrorActionPreference = 'Stop' }
        $result = Get-Content -LiteralPath $stderr -Raw
        if ($code -ne 1 -or !$result.Contains('load local endpoint config') -or !$result.Contains('intentionally-missing-client.yaml')) { throw "Unexpected result: $result" }
        if (!(Get-ChildItem -LiteralPath (Join-Path $repo '.tools/dzdeploy/build') -Recurse -Filter 'serverDZ.cfg')) { throw 'No configuration bundle extracted.' }
        Write-Host "${alias}: committed packaging and validation passed without Go or host access"
    }
} finally {
    $resolved = [IO.Path]::GetFullPath($base)
    if ((Split-Path $resolved -Parent) -ne [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd('\') -or (Split-Path $resolved -Leaf) -notlike 'dzdeploy-config-test-*') { throw 'Unsafe fixture cleanup path.' }
    Remove-Item -LiteralPath $resolved -Recurse -Force
}
$global:LASTEXITCODE = 0
