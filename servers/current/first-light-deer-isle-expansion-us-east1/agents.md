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
- When adding a patrol, set a meaningful unique `Name`. Current definitions use the `DI_<role>_<source>` convention.
- When changing patrol `Loadout`, verify the corresponding file exists in `profiles/ExpansionMod/Loadouts/<Loadout>.json`, unless the user intentionally wants an internal Expansion loadout.
- When changing factions, check the Expansion AI faction behavior first. Faction changes alter hostility and player interaction, not just labels.
- When changing AI quantity, remember load balancing. More entries do not necessarily mean more active AI if category caps remain unchanged.
- When changing `ROAMING_LOCAL` patrols, remember that only the first waypoint is used as the local roaming anchor.
- When changing `LOOP` patrols, preserve waypoint order because route order is the behavior.
- When changing spawn placement, use `[x, y, z]` coordinates. `y` is commonly `0.0` in this repo so terrain height is resolved by the game.

## Current Configuration Notes

- `AIPatrolSettings.json` has 84 named definitions: 43 civilian coordinate anchors and 41 object-class profiles.
- Eight role budgets allow 33 active patrols / 60 living AI across 0-255 players; these are ceilings, not guaranteed populations.
- All patrols use `ROAMING_LOCAL`. The 58 existing roaming locations are preserved; local roaming is not a hard site boundary.
- Seven stable `DI_*` selectors choose among 32 named archetypes (39 AI files total). The three `Player*Loadout` files remain unchanged.
- Each AI independently selects equipment. All chances are 1; repeated exclusive-group tickets encode weights, and empty tickets encode absence. Do not treat `Chance` as a normalized weight.
- NATO keeps woodland USMC clothing. Marksman is a 10% per-AI branch with FAL/M14, 4x ACOG and matching plate carrier/pouches. AugShort is excluded; standard Aug uses BUIS and TeamLeader uses a powered red dot.
- One full attached magazine plus one full spare accompanies magazine-fed weapons; internal weapons get two full native ammo stacks. Every doctor retains the complete baseline medical cargo.
- Players use West; NATO uses East; militia uses Mercenaries; police uses Guards; firefighters use Brawlers; civilians and medical staff use Civilian.
- Stock militia can attack civilian/medical AI. Brawlers retain their native melee bonus. No custom faction or healer mod is installed.
- Civilian and medical AI start unarmed but may scavenge weapons. All patrols use `UnlimitedReload: 6` (animals/infected only).
- Dedicated prison/ice-island/quest-route coverage is deferred. Configuration validation does not establish runtime placement, packing or combat behavior.
- Run `python scripts/audit-ai.py` from this server directory for the read-only policy audit; add `--write-report` only when deliberately refreshing evidence.
- `audit_ai_loadouts.py` checks include graphs, branch ownership, exact probabilities, mandatory supplies and conservative visible combinations. Run `python -m unittest discover -s scripts -p 'test_*.py'` for its selection tests. Server-root dzcli validation supplies the context needed for sibling includes.

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
