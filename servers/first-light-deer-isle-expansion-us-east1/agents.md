# Agent Instructions

This repository contains a DayZ server configuration for Deer Isle with DayZ Expansion AI. Treat the Expansion AI files as operational server configuration, not as generic example JSON.

## Important Paths

- Patrol definitions: `empty.deerisle/expansion/settings/AIPatrolSettings.json`
- Roaming locations: `empty.deerisle/expansion/settings/AILocationSettings.json`
- Global AI behavior: `profiles/ExpansionMod/Settings/AISettings.json`
- AI loadouts: `profiles/ExpansionMod/Loadouts/*.json`
- Expansion logs: `profiles/ExpansionMod/Logs/*.log`
- Human documentation: `docs/ai.md`

## Working Rules

- Read `docs/ai.md` before changing AI configuration.
- Preserve valid JSON. After edits, validate changed JSON files with PowerShell `ConvertFrom-Json` or an equivalent parser.
- Avoid wholesale reformatting of large Expansion JSON files unless the user explicitly asks for it. These files are large and hard to review when reordered or reformatted.
- Do not change `m_Version` fields unless the Expansion documentation or a migration requires it.
- Do not delete existing patrols, loadouts, or settings unless the user explicitly requests removal.
- When adding a patrol, set a meaningful `Name` when possible. Current patrols mostly have blank names, but new work should be easier to inspect.
- When changing patrol `Loadout`, verify the corresponding file exists in `profiles/ExpansionMod/Loadouts/<Loadout>.json`, unless the user intentionally wants an internal Expansion loadout.
- When changing factions, check the Expansion AI faction behavior first. Faction changes alter hostility and player interaction, not just labels.
- When changing AI quantity, remember load balancing. More entries do not necessarily mean more active AI if category caps remain unchanged.
- When changing `ROAMING_LOCAL` patrols, remember that only the first waypoint is used as the local roaming anchor.
- When changing `LOOP` patrols, preserve waypoint order because route order is the behavior.
- When changing spawn placement, use `[x, y, z]` coordinates. `y` is commonly `0.0` in this repo so terrain height is resolved by the game.

## Current Configuration Notes

- `AIPatrolSettings.json` has 130 patrol definitions.
- All patrols currently use `LoadBalancingCategory: "Patrol"`.
- The `Patrol` category currently caps active patrols at 5 for 0-255 players.
- Most patrols are `ROAMING_LOCAL`.
- A smaller number are fixed `LOOP` routes around oil rig/deepwater areas and Portland.
- One East patrol is `HALT`.
- The four Mercenary patrols use `Loadout: "WestLoadout"`. This resolves a previous `Loadout: "West"` reference that had no matching `West.json` file.

## Useful Validation Commands

Run these from the server directory.

Validate key JSON files:

```powershell
Get-Content -Raw empty.deerisle\expansion\settings\AIPatrolSettings.json | ConvertFrom-Json | Out-Null
Get-Content -Raw empty.deerisle\expansion\settings\AILocationSettings.json | ConvertFrom-Json | Out-Null
Get-Content -Raw profiles\ExpansionMod\Settings\AISettings.json | ConvertFrom-Json | Out-Null
Get-ChildItem profiles\ExpansionMod\Loadouts -Filter *.json | ForEach-Object {
  Get-Content -Raw $_.FullName | ConvertFrom-Json | Out-Null
}
```

Find missing patrol loadout files:

```powershell
$patrols = Get-Content -Raw empty.deerisle\expansion\settings\AIPatrolSettings.json | ConvertFrom-Json
$loadoutFiles = Get-ChildItem profiles\ExpansionMod\Loadouts -Filter *.json | ForEach-Object { $_.BaseName }
$patrols.Patrols.Loadout | Sort-Object -Unique | ForEach-Object {
  [pscustomobject]@{
    Loadout = $_
    FileExists = $loadoutFiles -contains $_
  }
}
```

Summarize patrols by faction, loadout, and behavior:

```powershell
$patrols = Get-Content -Raw empty.deerisle\expansion\settings\AIPatrolSettings.json | ConvertFrom-Json
$patrols.Patrols | Group-Object Faction, Loadout, Behaviour | Sort-Object Count -Descending
```

## Documentation Expectations

- If you make meaningful AI configuration changes, update `docs/ai.md`.
- Include the practical effect of changes, not just the fields changed.
- Mention whether the change affects spawn locations, faction hostility, patrol behavior, load balancing, or equipment.
- If there are risks or uncertain Expansion behaviors, say so directly and point to the relevant wiki/source.
