AI patrol structure (DayZ Expansion)
- Each patrol entry lives in `mpmissions/empty.sakhal/expansion/settings/AIPatrolSettings.json` under the `Patrols` array.
- Key fields: `Name` (your custom identifier), `Faction`, `Loadout`, `NumberOfAI` (or explicit `Units`), `Behaviour`, movement (`Speed`, `UnderThreatSpeed`, `DefaultStance`), spawn/despawn distances (`MinDistRadius`, `MaxDistRadius`, `DespawnRadius`), spread (`MinSpreadRadius`, `MaxSpreadRadius`), probabilities (`Chance`), and `Waypoints` (array of `[x, y, z]` coordinates). Most tuning fields accept `-1` to inherit from `AISettings.json`.
- Your custom patrols do have names (e.g., `Severomorsk_1`) which identify each entry.
- Reference docs: AIPatrolSettings guide and per-field meanings: https://github.com/salutesh/DayZ-Expansion-Scripts/wiki/%5BServer-Hosting%5D-AIPatrolSettings ; global defaults inherited when values are `-1`: https://github.com/salutesh/DayZ-Expansion-Scripts/wiki/%5BServer-Hosting%5D-AISettings
