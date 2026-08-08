# dzcli Use Cases

Use the installed binary's help as the source of truth for exact flags:

```powershell
& $dz --help
& $dz <command> --help
```

## PowerShell Patterns

Invoke a single command by splatting a named argument array:

```powershell
$argList = @("validate", "server", "servers/example/serverDZ.cfg")
& $dz @argList
```

When storing commands for a loop, avoid a property or parameter named `Args`; `$args` is a PowerShell automatic variable and array binding can become ambiguous. Prefer `ArgList` plus named parameters. If a mandatory parameter receives an initially empty collection, add `[AllowEmptyCollection()]`.

```powershell
function Add-DzcliCommand {
    param(
        [AllowEmptyCollection()][System.Collections.Generic.List[object]]$CommandList,
        [object[]]$ArgList
    )

    $CommandList.Add([pscustomobject]@{
        ArgList = [object[]]$ArgList
    }) | Out-Null
}

$commands = New-Object System.Collections.Generic.List[object]
Add-DzcliCommand -CommandList $commands -ArgList @("validate", "xml", $missionRoot)

foreach ($command in $commands) {
    $argList = [object[]]$command.ArgList
    & $dz @argList
}
```

If a batch validation prints the top-level `dzcli` help for every target, stop and test one command directly; `dzcli` likely received an empty or malformed argument list.

## Inspection Before Mutation

Use `get` commands to locate and inspect existing resources before editing:

- `get server --file <serverDZ.cfg>` lists supported server config fields.
- `get gameplay --file <cfggameplay.json>` lists gameplay fields by dotted path.
- `get economy types [name] --file <types.xml>` inspects one economy file.
- `get economy types [name] --cfgeconomycore <cfgeconomycore.xml>` resolves the unique types file in multi-file missions.
- `get economy types --cfgeconomycore <cfgeconomycore.xml> --duplicates` finds duplicate type names.
- `get economy events [name] --file <db/events.xml>` inspects event activity and positioning.
- `get economy event-spawns [name] --file <cfgeventspawns.xml>` inspects fixed spawn positions and zones.
- `get economy environment [territory] --file <cfgenvironment.xml>` inspects territory path and usable references.
- `get economy limits <category|tag|usage|value> [name] --file <cfglimitsdefinition.xml>` inspects base limits.
- `get expansion ai patrols [name] --file <AIPatrolSettings.json>` inspects patrols.
- `get expansion ai loadouts [name] --path <server-root>` or `--file <loadout.json>` inspects loadouts.

## Mutations

Preview supported mutations with `--dry-run`, then apply the same command without `--dry-run`.

- Server config: `update server <field> --file <serverDZ.cfg> --value <value> [--force]`; `delete server <field>`.
- Gameplay config: `update gameplay <field> --file <cfggameplay.json> --value <value> [--force]`; repeat `--value` for arrays; use `--clear` to empty arrays; `delete gameplay <field>`.
- Economy types: `update economy types <name> --file <types.xml>` or `--cfgeconomycore <cfgeconomycore.xml>` with scalar, flag, category, tag, usage, value, rename, or occurrence flags.
- Economy event spawns: `create/update/delete economy event-spawns <name> --file <cfgeventspawns.xml>` with `--pos`, `--add-pos`, `--set-pos`, `--remove-pos`, `--zone`, `--set-zone`, `--remove-zone`, or `--copy-zone-from`.
- Economy environment references: `create/update/delete economy environment path|usable ... --file <cfgenvironment.xml>`. Use scaffolding flags only when an empty or template territory file is acceptable.
- Limits: `create/delete economy limits ...` and `create/delete economy limits group ...`.
- Expansion AI patrols: `create/update/delete expansion ai patrols ... --file <AIPatrolSettings.json>`.
- Expansion AI loadouts: `create/update/delete expansion ai loadouts ... --file <loadout.json>`, including nested `loadouts item` commands for cargo, sets, and attachments.
- Economy remediation: `fix economy <mission-root|economy-file>` previews an ordered plan; `--apply` runs only supported unambiguous fixes; `--allow-destructive` is required for deterministic deletions.

## Validation Map

Always validate after a modification, whether it was made by `dzcli` or by direct file edit:

| Changed target | Validation command |
| --- | --- |
| `serverDZ.cfg` | `dzcli validate server <serverDZ.cfg>` |
| `cfggameplay.json` | `dzcli validate gameplay <cfggameplay.json>` |
| Mission economy folder or economy XML | `dzcli validate economy <mission-root-or-file>` |
| Any XML tree | `dzcli validate xml <path>` |
| `cfgweather.xml` | `dzcli validate weather <cfgweather.xml>` |
| `init.c` | `dzcli validate init <init.c>` |
| Expansion AI settings, patrols, locations, loadouts, or loot drops | `dzcli validate expansion ai <server-or-repo-root>` |

Economy validation warnings may print scoped remediation commands. Preview those commands with `--dry-run` when supported. A `validation-only` notice means `dzcli` cannot safely perform the edit and a manual change is required.

Run baseline validation before edits when practical. If baseline validation already fails, keep the output and rerun the same validation after the requested change. Fix pre-existing failures only when they are part of the task or block safe completion.

For a read-only repository-wide validation report, run:

```powershell
.\skills\dzcli\scripts\validate-all.ps1
.\skills\dzcli\scripts\validate-all.ps1 -Json
```

The helper discovers `serverDZ.cfg`, root and mission config files, server-wide XML trees, and Expansion AI roots under `servers/`. It reports skipped economy checks when a partial mission folder contains economy XML but lacks `cfgeconomycore.xml`, because the current `dzcli validate economy` behavior cannot validate those folders cleanly.

## Direct Edit Escapes

Use direct edits when the desired change is outside the command surface, such as unsupported weather modifications, mission script changes, comments, or complex mod-management batch scripts. Keep direct edits local and structured, then run the closest validation command before finishing.
