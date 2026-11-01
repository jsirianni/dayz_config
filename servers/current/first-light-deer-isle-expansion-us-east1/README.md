# First Light Deer Isle Expansion

## Configuration reset and restore notes

When resetting the `empty.deerisle` mission from Deer Isle Stable V5.9 defaults, preserve the following server-specific configuration so the vehicle, Terje, and Expansion AI setups can be restored.

### Custom vehicles (ToyotaRunner / 4Runner)

These existing mission files contain the ToyotaRunner vehicle event, variants, spawn locations, or spawnable type definitions:

- `empty.deerisle/db/types.xml` — economy entries for six ToyotaRunner variants.
- `empty.deerisle/db/events.xml` — the `VehicleToyotaRunner` event and its variants.
- `empty.deerisle/cfgeventspawns.xml` — ToyotaRunner spawn coordinates.
- `empty.deerisle/cfgspawnabletypes.xml` — vehicle spawnable type definitions.

### Terje medicine

- `empty.deerisle/cfgeconomycore.xml` — registers the Terje type definitions (and Expansion CE files).
- `empty.deerisle/modded_types/terje.xml` — Terje medicine and item economy entries; this is an added file, not a V5.9 default.

Keep the `cfgeconomycore.xml` registration together with `modded_types/terje.xml`; the file alone is not loaded unless registered.

### DayZ Expansion AI

Expansion AI patrol configuration and loadouts are custom files, rather than edits to the V5.9 default mission files:

- `empty.deerisle/expansion/settings/AIPatrolSettings.json` — patrol definitions and loadout/faction selections.
- `empty.deerisle/expansion/settings/AILocationSettings.json` — local roaming locations used by patrols.
- `profiles/ExpansionMod/Settings/AISettings.json` — global Expansion AI behavior settings.
- `profiles/ExpansionMod/Loadouts/DI_*.json` — custom Deer Isle AI loadout selectors and archetypes.

The profile files are outside `empty.deerisle`; include them in any broader profile reset or restore plan.

### Reset scope

This is a preservation checklist, not a full backup manifest. A wholesale replacement of `empty.deerisle` can also remove added Expansion CE/settings files and other server-specific changes. Preserve a copy of the full mission and relevant `profiles/ExpansionMod` files before resetting, then restore only the intended settings and validate the resulting mission configuration.
