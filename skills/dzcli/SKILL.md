---
name: dzcli
description: Install, update, and use dzcli as the first-choice tool for DayZ server configuration work. Use when Codex is working in this repository or on DayZ files such as serverDZ.cfg, cfggameplay.json, cfgweather.xml, init.c, central economy XML, cfgeconomycore.xml, types.xml, events.xml, cfgeventspawns.xml, cfgenvironment.xml, cfglimitsdefinition XML, or DayZ Expansion AI JSON loadout and patrol files.
---

# dzcli

## Overview

Use `dzcli` before manual edits whenever the task touches supported DayZ configuration files. It provides scoped `get`, `create`, `update`, `delete`, `fix`, and `validate` commands for large XML/JSON/config files that are easy to damage with broad text edits.

## Bootstrap

From the repository root, install or update the repo-local binary before using `dzcli`:

```powershell
.\skills\dzcli\scripts\install-dzcli.ps1
$dz = Join-Path (Get-Location) ".tools\dzcli\dzcli.exe"
& $dz version
```

The installer checks the installed version, queries the latest GitHub release, and overwrites the binary only when a newer release exists or `-Force` is supplied. It downloads the raw release binary directly, not a source checkout or archive.

If running outside PowerShell, still use the installed binary at `.tools/dzcli/dzcli.exe` on Windows. On non-Windows hosts, read the installer first and adapt the binary path it reports.

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
