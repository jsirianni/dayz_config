---
name: dzcli
description: Install, update, and use dzcli as the first-choice tool for DayZ server configuration work. Use when Codex is working in this repository or on DayZ files such as serverDZ.cfg, cfggameplay.json, cfgweather.xml, init.c, central economy XML, cfgeconomycore.xml, types.xml, events.xml, cfgeventspawns.xml, cfgenvironment.xml, cfglimitsdefinition XML, or DayZ Expansion AI JSON loadout and patrol files.
---

# dzcli

## Overview

Use `dzcli` before manual edits whenever the task touches supported DayZ configuration files. It provides scoped `get`, `create`, `update`, `delete`, `fix`, and `validate` commands for large XML/JSON/config files that are easy to damage with broad text edits.

## Bootstrap

From the repository root, install or update the repo-local binary before using `dzcli`:

Preferred Go workflow (no PowerShell dependency):

```text
go run ./cmd/dzdeploy tools install
go run ./cmd/dzdeploy validate all
```

Use `validate <alias>` for `nasdara`, `nasdara-staging`, or `deer-isle`. For local checks when release checking is unavailable, `validate <alias|all> --offline` explicitly uses cached dzcli; deployment never permits it. The Go validator discovers matching checks, reports warnings, and exits nonzero on failures or missing validators. It never edits configs.

The executable is `.tools/dzcli/dzcli.exe` on Windows and `.tools/dzcli/dzcli` elsewhere. Invoke it directly for get/mutations, dry-run previews, and matching post-change validation. The Go installer checks the release and verifies its SHA-256 digest. If macOS has no release asset, initial installation builds the released module with Go checksum verification; that step requires Go.

The legacy PowerShell workflow also remains supported:

```powershell
.\skills\dzcli\scripts\install-dzcli.ps1
$dz = Join-Path (Get-Location) ".tools\dzcli\dzcli.exe"
& $dz version
```

The installer checks the installed version, queries the latest GitHub release, and overwrites the binary only when a newer release exists or `-Force` is supplied. It downloads the raw release binary directly, not a source checkout or archive.

If running outside PowerShell, still use the installed binary at `.tools/dzcli/dzcli.exe` on Windows. On non-Windows hosts, read the installer first and adapt the binary path it reports.

## PowerShell Invocation Guardrails

When scripting multiple `dzcli` calls in PowerShell, keep command arguments in a plainly named array and splat that variable directly:

```powershell
$argList = @("validate", "server", "servers/example/serverDZ.cfg")
& $dz @argList
```

For batch harnesses, use a property named `ArgList`, not `Args`, and recover it as `[object[]]` before invocation:

```powershell
$command = [pscustomobject]@{
    ArgList = [object[]]@("validate", "economy", $missionRoot)
}
$argList = [object[]]$command.ArgList
& $dz @argList
```

Avoid hand-rolled loops that pass array literals positionally into functions or use a property/parameter named `Args`; PowerShell can bind or expand those arrays in surprising ways and leave `dzcli` receiving no subcommand, which prints top-level help and can look like a false pass. For repository-wide validation, prefer the maintained helper:

```powershell
.\skills\dzcli\scripts\validate-all.ps1
.\skills\dzcli\scripts\validate-all.ps1 -Json
```

## Default Workflow

1. Identify the server root under `servers/<server-name>` and the specific mission/config file.
2. Run `dzcli --help` or command-specific help when flags are uncertain.
3. Inspect current state with `get` commands before editing.
4. Validate the relevant file or mission before changing it when possible so pre-existing failures are visible.
5. Preview every supported mutation with `--dry-run`.
6. Apply the same command without `--dry-run` after reviewing the preview. Use `--force` for non-interactive `server` and `gameplay` overwrites when needed.
7. Run the matching validation command after every change, including changes made by direct file edit.

Direct file edits are acceptable only when `dzcli` does not expose a sufficient command for the change. After direct edits, still run the closest `dzcli validate ...` command.

If baseline validation already fails, record the exact output, make only the requested change, and rerun validation afterward. Do not claim the task is clean unless validation passes; distinguish pre-existing failures from any new failures.

## Command Selection

Use `references/use-cases.md` for a compact command map and examples. Trust the installed command help if it differs from these notes:

```powershell
& $dz --help
& $dz update economy types --help
& $dz validate economy --help
```

Prefer these command families:

- `serverDZ.cfg`: `get server`, `update server`, `delete server`, then `validate server`.
- `cfggameplay.json`: `get gameplay`, `update gameplay`, `delete gameplay`, then `validate gameplay`.
- Central economy XML: `get/create/update/delete economy ...`, `fix economy`, then `validate economy` and often `validate xml`.
- `cfgweather.xml`: direct edit if needed, then `validate weather`.
- `init.c`: direct edit if needed, then `validate init`.
- Expansion AI JSON: `get/create/update/delete expansion ai ...`, then `validate expansion ai`.

## Repository Patterns

Mission roots in this repo vary by server. Locate them by finding `db/types.xml`, `cfgeconomycore.xml`, `cfggameplay.json`, or `cfgweather.xml` under the target server directory.

For validating every server directory, use:

```powershell
.\skills\dzcli\scripts\validate-all.ps1
```

The script discovers supported config targets, invokes `dzcli` read-only validators, reports warnings and failures, validates server-wide XML trees so loose XML is not missed, and skips economy validation for partial mission folders that have economy XML but no `cfgeconomycore.xml` because the current `dzcli validate economy` command requires mission core discovery.

With dzcli v0.0.5 or newer, validation supports compact warning output and JSON:

```powershell
& $dz validate economy $mission --warnings compact
& $dz -o json validate server "$server/serverDZ.cfg"
```

Prefer compact text for high-warning economy trees until JSON warning compaction is available; JSON currently preserves full warning/remediation detail.

Example server config change:

```powershell
$server = "servers/first-light-deer-isle-hardcore-us-east1"
& $dz get server hostname --file "$server/serverDZ.cfg"
& $dz update server hostname --file "$server/serverDZ.cfg" --value "New Hostname" --dry-run
& $dz update server hostname --file "$server/serverDZ.cfg" --value "New Hostname" --force
& $dz validate server "$server/serverDZ.cfg"
```

Example economy type change:

```powershell
$mission = "servers/first-light-namalsk-survival-us-east/mpmissions/hardcore.namalsk"
& $dz validate economy $mission
& $dz get economy types M4A1 --cfgeconomycore "$mission/cfgeconomycore.xml"
& $dz update economy types M4A1 --cfgeconomycore "$mission/cfgeconomycore.xml" --nominal 20 --min 5 --dry-run
& $dz update economy types M4A1 --cfgeconomycore "$mission/cfgeconomycore.xml" --nominal 20 --min 5
& $dz validate economy $mission
& $dz validate xml $mission
```
