param(
    [string]$ServersDir,
    [string]$DzcliPath,
    [switch]$Json,
    [switch]$IncludeOutput,
    [switch]$SkipInstall
)

$ErrorActionPreference = "Continue"

function Get-RepoRoot {
    $candidate = Join-Path $PSScriptRoot "..\..\.."
    return (Resolve-Path -LiteralPath $candidate).ProviderPath
}

function Convert-ToRepoRelativePath {
    param(
        [string]$Path,
        [string]$RepoRoot
    )

    $resolved = Resolve-Path -LiteralPath $Path -ErrorAction SilentlyContinue
    if (-not $resolved) {
        return $Path
    }

    $fullPath = $resolved.ProviderPath
    $prefix = $RepoRoot.TrimEnd("\", "/") + [System.IO.Path]::DirectorySeparatorChar
    if ($fullPath.StartsWith($prefix, [System.StringComparison]::OrdinalIgnoreCase)) {
        return $fullPath.Substring($prefix.Length)
    }

    return $fullPath
}

function Convert-OutputPath {
    param(
        [string]$Line,
        [string]$RepoRoot
    )

    return $Line.Replace($RepoRoot.TrimEnd("\", "/") + "\", "").Replace($RepoRoot.TrimEnd("\", "/") + "/", "")
}

function Add-ValidationCommand {
    param(
        [Parameter(Mandatory = $true)][AllowEmptyCollection()][System.Collections.Generic.List[object]]$CommandList,
        [Parameter(Mandatory = $true)][string]$Server,
        [Parameter(Mandatory = $true)][string]$Kind,
        [Parameter(Mandatory = $true)][object[]]$ArgList,
        [Parameter(Mandatory = $true)][string]$Target
    )

    $CommandList.Add([pscustomobject]@{
        Server = $Server
        Kind = $Kind
        ArgList = [object[]]$ArgList
        Target = $Target
    }) | Out-Null
}

function Add-SkippedValidation {
    param(
        [Parameter(Mandatory = $true)][AllowEmptyCollection()][System.Collections.Generic.List[object]]$SkipList,
        [Parameter(Mandatory = $true)][string]$Server,
        [Parameter(Mandatory = $true)][string]$Kind,
        [Parameter(Mandatory = $true)][string]$Target,
        [Parameter(Mandatory = $true)][string]$Reason
    )

    $SkipList.Add([pscustomobject]@{
        Server = $Server
        Kind = $Kind
        Target = $Target
        Reason = $Reason
    }) | Out-Null
}

function Get-MissionRoots {
    param([string]$ServerDir)

    Get-ChildItem -LiteralPath $ServerDir -Recurse -Directory -ErrorAction SilentlyContinue |
        Where-Object {
            (Test-Path -LiteralPath (Join-Path $_.FullName "db\types.xml")) -or
            (Test-Path -LiteralPath (Join-Path $_.FullName "cfgeconomycore.xml")) -or
            (Test-Path -LiteralPath (Join-Path $_.FullName "cfggameplay.json")) -or
            (Test-Path -LiteralPath (Join-Path $_.FullName "cfgweather.xml")) -or
            (Test-Path -LiteralPath (Join-Path $_.FullName "init.c"))
        } |
        Sort-Object FullName -Unique
}

function Test-ExpansionAiConfig {
    param([string]$ServerDir)

    $paths = @(
        "profiles\ExpansionMod\AI",
        "profiles\ExpansionMod\Loadouts",
        "empty.deerisle\expansion\settings\AIPatrolSettings.json",
        "mpmissions\hardcore.namalsk\expansion\settings\AIPatrolSettings.json"
    )

    foreach ($path in $paths) {
        if (Test-Path -LiteralPath (Join-Path $ServerDir $path)) {
            return $true
        }
    }

    return $false
}

function Get-Status {
    param(
        [int]$ExitCode,
        [int]$WarningCount
    )

    if ($ExitCode -ne 0) {
        return "FAIL"
    }

    if ($WarningCount -gt 0) {
        return "WARN"
    }

    return "PASS"
}

function Write-MarkdownReport {
    param(
        [object]$Summary
    )

    Write-Output "# dzcli Validation Report"
    Write-Output ""
    Write-Output "dzcli: $($Summary.DzcliPath)"
    Write-Output "Servers: $($Summary.ServerCount)"
    Write-Output "Commands: $($Summary.CommandCount)"
    Write-Output "Passed: $($Summary.PassCount)"
    Write-Output "Warnings: $($Summary.WarnCount)"
    Write-Output "Failed: $($Summary.FailCount)"
    Write-Output "Skipped: $($Summary.SkipCount)"
    Write-Output ""

    $issues = @($Summary.Results | Where-Object { $_.Status -ne "PASS" })
    if ($issues.Count -eq 0 -and $Summary.Skipped.Count -eq 0) {
        Write-Output "All discovered validations passed."
        return
    }

    if ($issues.Count -gt 0) {
        Write-Output "## Issues"
        foreach ($result in $issues) {
            Write-Output ""
            Write-Output "### $($result.Status): $($result.Server) / $($result.Kind)"
            Write-Output ""
            Write-Output "Target: $($result.Target)"
            Write-Output "Warnings: $($result.WarningCount); failures: $($result.FailureCount)"
            foreach ($line in $result.IssueSample) {
                Write-Output "- $line"
            }
            if ($result.OutputLineCount -gt $result.IssueSample.Count) {
                Write-Output "- Output lines: $($result.OutputLineCount); sample shown above."
            }
        }
        Write-Output ""
    }

    if ($Summary.Skipped.Count -gt 0) {
        Write-Output "## Skipped"
        foreach ($skip in $Summary.Skipped) {
            Write-Output "- $($skip.Server) / $($skip.Kind): $($skip.Target) - $($skip.Reason)"
        }
    }
}

$repoRoot = Get-RepoRoot

if ([string]::IsNullOrWhiteSpace($ServersDir)) {
    $ServersDir = Join-Path $repoRoot "servers"
}

if ([string]::IsNullOrWhiteSpace($DzcliPath)) {
    $DzcliPath = Join-Path (Join-Path $repoRoot ".tools") "dzcli\dzcli.exe"
}

if (-not $SkipInstall) {
    & (Join-Path $PSScriptRoot "install-dzcli.ps1") | Out-Null
}

if (-not (Test-Path -LiteralPath $DzcliPath)) {
    throw "dzcli not found at $DzcliPath. Run skills\dzcli\scripts\install-dzcli.ps1 first, or pass -DzcliPath."
}

if (-not (Test-Path -LiteralPath $ServersDir)) {
    throw "Servers directory not found: $ServersDir"
}

$commands = New-Object System.Collections.Generic.List[object]
$skipped = New-Object System.Collections.Generic.List[object]
$serverDirs = @(Get-ChildItem -LiteralPath $ServersDir -Directory | Sort-Object Name)

foreach ($serverDirInfo in $serverDirs) {
    $serverDir = $serverDirInfo.FullName
    $serverName = $serverDirInfo.Name
    $serverCfg = Join-Path $serverDir "serverDZ.cfg"

    if (Test-Path -LiteralPath $serverCfg) {
        Add-ValidationCommand -CommandList $commands -Server $serverName -Kind "server" -ArgList @("validate", "server", $serverCfg) -Target $serverCfg
    }

    $rootGameplay = Join-Path $serverDir "cfggameplay.json"
    if (Test-Path -LiteralPath $rootGameplay) {
        Add-ValidationCommand -CommandList $commands -Server $serverName -Kind "gameplay" -ArgList @("validate", "gameplay", $rootGameplay) -Target $rootGameplay
    }

    $rootWeather = Join-Path $serverDir "cfgweather.xml"
    if (Test-Path -LiteralPath $rootWeather) {
        Add-ValidationCommand -CommandList $commands -Server $serverName -Kind "weather" -ArgList @("validate", "weather", $rootWeather) -Target $rootWeather
    }

    $rootInit = Join-Path $serverDir "init.c"
    if (Test-Path -LiteralPath $rootInit) {
        Add-ValidationCommand -CommandList $commands -Server $serverName -Kind "init" -ArgList @("validate", "init", $rootInit) -Target $rootInit
    }

    foreach ($mission in @(Get-MissionRoots $serverDir)) {
        $missionRoot = $mission.FullName
        $cfgeconomycore = Join-Path $missionRoot "cfgeconomycore.xml"
        $hasEconomyFiles = (Test-Path -LiteralPath (Join-Path $missionRoot "db\types.xml")) -or
            (Test-Path -LiteralPath (Join-Path $missionRoot "db\economy.xml")) -or
            (Test-Path -LiteralPath $cfgeconomycore)

        if (Test-Path -LiteralPath $cfgeconomycore) {
            Add-ValidationCommand -CommandList $commands -Server $serverName -Kind "economy" -ArgList @("validate", "economy", $missionRoot) -Target $missionRoot
        } elseif ($hasEconomyFiles) {
            Add-SkippedValidation -SkipList $skipped -Server $serverName -Kind "economy" -Target $missionRoot -Reason "cfgeconomycore.xml is missing; dzcli validate economy currently requires mission core discovery."
        }

        $gameplay = Join-Path $missionRoot "cfggameplay.json"
        if (Test-Path -LiteralPath $gameplay) {
            Add-ValidationCommand -CommandList $commands -Server $serverName -Kind "gameplay" -ArgList @("validate", "gameplay", $gameplay) -Target $gameplay
        }

        $weather = Join-Path $missionRoot "cfgweather.xml"
        if (Test-Path -LiteralPath $weather) {
            Add-ValidationCommand -CommandList $commands -Server $serverName -Kind "weather" -ArgList @("validate", "weather", $weather) -Target $weather
        }

        $init = Join-Path $missionRoot "init.c"
        if (Test-Path -LiteralPath $init) {
            Add-ValidationCommand -CommandList $commands -Server $serverName -Kind "init" -ArgList @("validate", "init", $init) -Target $init
        }
    }

    $xmlFiles = @(Get-ChildItem -LiteralPath $serverDir -Recurse -File -Filter "*.xml" -ErrorAction SilentlyContinue)
    if ($xmlFiles.Count -gt 0) {
        Add-ValidationCommand -CommandList $commands -Server $serverName -Kind "xml" -ArgList @("validate", "xml", $serverDir) -Target $serverDir
    }

    if (Test-ExpansionAiConfig $serverDir) {
        Add-ValidationCommand -CommandList $commands -Server $serverName -Kind "expansion-ai" -ArgList @("validate", "expansion", "ai", $serverDir) -Target $serverDir
    }
}

$results = foreach ($command in $commands) {
    $argList = [object[]]$command.ArgList
    $outputLines = @(& $DzcliPath @argList 2>&1 | ForEach-Object { Convert-OutputPath -Line $_.ToString() -RepoRoot $repoRoot })
    $exitCode = $LASTEXITCODE
    $warnings = @($outputLines | Where-Object { $_ -match "\bwarning:" })
    $failures = @($outputLines | Where-Object { $_ -match "\bfailed:|\bfailed\b|error:" })
    $issueSample = @($failures + $warnings | Select-Object -First 20)

    [pscustomobject]@{
        Server = $command.Server
        Kind = $command.Kind
        Target = Convert-ToRepoRelativePath -Path $command.Target -RepoRoot $repoRoot
        ExitCode = $exitCode
        Status = Get-Status -ExitCode $exitCode -WarningCount $warnings.Count
        WarningCount = $warnings.Count
        FailureCount = $failures.Count
        IssueSample = $issueSample
        OutputLineCount = $outputLines.Count
        Output = if ($IncludeOutput) { $outputLines } else { @() }
    }
}

$summary = [pscustomobject]@{
    DzcliPath = Convert-ToRepoRelativePath -Path $DzcliPath -RepoRoot $repoRoot
    ServersDir = Convert-ToRepoRelativePath -Path $ServersDir -RepoRoot $repoRoot
    ServerCount = $serverDirs.Count
    CommandCount = $commands.Count
    PassCount = @($results | Where-Object { $_.Status -eq "PASS" }).Count
    WarnCount = @($results | Where-Object { $_.Status -eq "WARN" }).Count
    FailCount = @($results | Where-Object { $_.Status -eq "FAIL" }).Count
    SkipCount = $skipped.Count
    Results = @($results)
    Skipped = @($skipped | ForEach-Object {
        [pscustomobject]@{
            Server = $_.Server
            Kind = $_.Kind
            Target = Convert-ToRepoRelativePath -Path $_.Target -RepoRoot $repoRoot
            Reason = $_.Reason
        }
    })
}

if ($Json) {
    $summary | ConvertTo-Json -Depth 8
} else {
    Write-MarkdownReport -Summary $summary
}
