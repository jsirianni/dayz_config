# Nasdara Expansion AI Implementation

Last updated: 2026-10-19

Status: the approved configuration phase is implemented: 4 Civilian loadouts,
6 Tribesman loadouts, and 40 single-AI patrols at 20 copied official spawn-area
centers. Static validation passes. Deployment, runtime acceptance, the scanner,
Warlords, and Western_Merc remain pending.

## Objective and scope

The longer-term scanner goal is to produce hundreds of useful Expansion AI
spawn locations for Nasdara through an automated batch process. The operator
must not have to inspect, teleport to, or
approve individual coordinates. Locations must provide sufficiently flat,
outdoor ground, correct elevation, character clearance, and a usable walking
exit. Validate complete group placements as well as their anchors.

The intended workflow uses one local DayZ client session with Nasdara loaded in
DayZ Editor. It does not require launching a dedicated server for the geometry
scan. A custom scanner still needs to be implemented and checked against the
installed game, Editor, and map versions. Actual Expansion AI spawning and
behavior are a later validation concern, distinct from geometry acceptance.

Nasdara is the target for all implementation work in this plan. Patrol and
equipment patterns from other servers may inform design; their coordinates
must not be transferred to Nasdara.

The current phase uses the user's approved configuration-only alternative:
copy the official player spawn centers and let stock Expansion resolve height
and apply small spawn spreads. No new coordinates, geometry scanner, custom
mod, launcher changes, dependency installation, or game/server launch are part
of this phase. The centers have not passed the scanner acceptance rules below.

## Current evidence and constraints

- The launchers currently load CF and VPPAdminTools. Expansion AI configuration
  and its deployment prerequisites are documented, but the mods have not been
  added to the launchers or installed. See [README.md](README.md#expansion-ai-configuration).
- The checked-in mission is an override of an installed stock Nasdara mission;
  it now contains `db/messages.xml` and `expansion/settings/AIPatrolSettings.json`.
  Obtain the complete matching mission data for analysis rather than assuming
  it is in this checkout. This phase pins the official source at upstream
  commit `f220b2918b1d79a5055b1ee9be70c0ec1516e876`.
- The upstream Nasdara files inspected during planning contain 333 vehicle
  event positions and 20 fresh-player spawn area centers. These are search
  seeds, not verified AI spawn points. Recalculate these counts when choosing
  the implementation's source snapshot.
- Player spawn centers describe search grids. Object placements and loot
  layouts provide useful context but are not complete collision geometry or
  a terrain heightmap. Object origins and loot positions are not safe AI
  standing positions by themselves.
- Current upstream Expansion placement code can resolve against buildings,
  vehicles, roads, terrain, and other objects. Setting waypoint height to zero
  alone does not establish outdoor ground placement.
- DayZ Editor supports script execution and discovers installed worlds from
  `CfgWorlds`. Nasdara compatibility and reliable unattended scans have not
  been demonstrated locally.
- DayZ exposes navmesh sampling and path queries with movement filters. Their
  availability and behavior in the selected offline setup must be established.
- No scanner or in-game validation has been run. Geometry acceptance will
  describe the scanned map and object configuration; later vehicles, player
  construction, or changed map objects can alter clearance.

## Decisions already made

- Automate acceptance, rejection, retries, replacement candidates, and exports.
- Treat missing or inconclusive checks as unresolved, retry them, then reject
  them with a reason if they cannot be completed. Never silently skip a
  required check or count an unresolved location as accepted.
- For future scanner-accepted positions, begin with fixed, checked placement
  and zero random spread, accounting for formation offsets separately. The
  current unverified-center phase explicitly supersedes that choice with
  Civilian 1-2 m and Tribesman 4-5 m native spread and one AI per patrol.
- Preserve source provenance, map/build information, configuration, results,
  and checkpoints so a scan can be audited and resumed.
- Keep target counts and area quotas configurable. A request for 300 accepted
  locations is an example, not an agreed requirement.
- Report an explicit shortfall when an area's search budget is exhausted.
  Individual rejected points do not become operator review tasks.

## Implemented configuration phase

### Files and pinned inputs

| File or directory, relative to this server | Purpose |
| --- | --- |
| `profiles/ExpansionMod/Loadouts/Nasdara_Civilian_01.json` through `04.json` | Four tool-only starting loadouts |
| `profiles/ExpansionMod/Loadouts/Nasdara_Tribesman_01.json` through `06.json` | Six firearm starting loadouts |
| `profiles/ExpansionMod/Settings/AISettings.json` | Complete AI settings schema version 20, West players, recruitment disabled |
| `mpmissions/dayzOffline.nasdara/expansion/settings/AIPatrolSettings.json` | Complete patrol settings schema version 32 and 40 patrol definitions; no example patrols |
| `validation/baseline-expansion-ai.txt` | Exact pre-change validator failure: no AI configuration existed |
| `validation/final-expansion-ai.txt` | Final profile and mission validator output |
| `validation/expansion-ai-static-audit.json` | Static requirement audit, source checksums, counts and loadout assignments |

All inventory references use the official Nasdara
[types.xml](https://github.com/BohemiaInteractive/DayZ-Central-Economy/blob/f220b2918b1d79a5055b1ee9be70c0ec1516e876/dayzOffline.nasdara/db/types.xml)
at CE commit `f220b2918b1d79a5055b1ee9be70c0ec1516e876` as an explicit allowlist.
Other maps' catalogs and unlisted engine classes are excluded from these
loadouts. The pinned types snapshot has SHA-256
`b9efd7cbe36675faec703c4ecc963aa246f404b4367c234b073217c5ec6181fa`.

Settings fields and defaults were inspected at Expansion `release` commit
`0d26645ca31bc574bc3ebee1d4bc9192727f3c43`:
[AI settings](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/0d26645ca31bc574bc3ebee1d4bc9192727f3c43/DayZExpansion/AI/Scripts/3_Game/DayZExpansion_AI/Settings/ExpansionAISettings.c),
[patrol settings](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/0d26645ca31bc574bc3ebee1d4bc9192727f3c43/DayZExpansion/AI/Scripts/3_Game/DayZExpansion_AI/Settings/Patrols/ExpansionAIPatrolSettings.c),
and [patrol fields](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/0d26645ca31bc574bc3ebee1d4bc9192727f3c43/DayZExpansion/AI/Scripts/3_Game/DayZExpansion_AI/Settings/Patrols/ExpansionAIPatrolBase.c).
Serialized defaults are included; internal `NonSerialized` fields are omitted.
Accuracy, damage, aggression timeouts (120/150 seconds), lighting visibility,
and other unrelated settings retain stock defaults. Patrol override sentinels
inherit the root/global defaults where appropriate.

### Starting outfits and supplies

| Loadout suffix | Body | Legs | Tool or firearm | Optional supplies, independent chance |
| --- | --- | --- | --- | --- |
| Civilian_01 | `KameezShirt_Beige` | `MensShalwarPants_Brown` | `DonerKnife` | `WaterBottle` 30%; `BandageDressing` 25% |
| Civilian_02 | `ShortSleeveShirt_Green` | `MensShalwarPants_Grey` | `Hatchet` | None |
| Civilian_03 | `KameezShirt_NavyBlue` | `MensShalwarPants_White` | `Pipe` | `GlassBottle` 35%; `Matchbox` 20% |
| Civilian_04 | `KameezShirt_White` | `MensShalwarPants_Beige` | `KitchenKnife` | `BandageDressing` 40% |
| Tribesman_01 | `KameezShirt_Beige` | `MensShalwarPants_Brown` | `Izh18Shotgun` | `WaterBottle` 30% |
| Tribesman_02 | `KameezShirt_Olive` | `MensShalwarPants_Grey` | `Izh43Shotgun` | None |
| Tribesman_03 | `KameezShirt_NavyBlue` | `MensShalwarPants_NavyBlue` | `Mosin9130` | `BandageDressing` 30%; `GlassBottle` 20% |
| Tribesman_04 | `KameezShirt_Grey` | `MensShalwarPants_White` | `Mosin9130` | `WaterBottle` 20%; `Matchbox` 15% |
| Tribesman_05 | `KameezShirt_Brown` | `MensShalwarPants_Beige` | `SKS` | None |
| Tribesman_06 | `OBRJacket_Olive` | `OBRPants_Olive` | `SKS_Desert` | `BandageDressing` 35% |

Each filename starts with `Nasdara_` and ends with `.json`. Tools and supplies
use root cargo; firearms use the `Shoulder` slot. All starting items are pristine.
Civilians receive no starting firearm, ammunition, explosives, or second weapon.

Each clothing slot selects one guaranteed candidate uniformly through stock
Expansion attachment selection: all alternatives have `Chance: 1`.

- Headgear: `PakolHat_Beige`, `PakolHat_Brown`, `PakolHat_Grey`,
  `TurbanHat_Beige`, `TurbanHat_White`; Tribesman_06 instead always uses
  `OBRCap_Olive`.
- Feet: `SandalsShoes_Brown`, `SandalsShoes_Black`,
  `TraditionalSandalShoes_LightBrown`.
- Civilian Vest: `LinenVest_Brown`, `LinenVest_Grey`, `RobeVest_Beige`.
- Tribesman Vest: `ChestRigVest_Khaki`, `ChestRigVest_Olive`, `ChestRigVest_Tan`.

Supplies are separate cargo entries, each rolled once with its own chance.
Bottles use normalized quantities 0.35-0.65 and native liquid defaults. Other
supplies use 0/0, preserving native initial quantities. No bottles or supplies
are guaranteed and the three loadouts marked None have no helper supplies.

Each Tribesman also has one guaranteed loose-ammo stack; there are no ammo boxes
or extra ammo types:

| Firearm | Ammo class | Quantity Min/Max | Intended initial rounds for a 20-round stack |
| --- | --- | --- | --- |
| Izh18/Izh43 | `Ammo_12gaPellets` | 0.20 / 0.20 | About 4 |
| Mosin | `Ammo_762x54` | 0.25 / 0.25 | About 5 |
| SKS/SKS_Desert | `Ammo_762x39` | 0.40 / 0.40 | About 8 |

Expansion prefab quantities are fractions of maximum capacity, as shown by
the pinned [prefab implementation](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/0d26645ca31bc574bc3ebee1d4bc9192727f3c43/DayZExpansion/Core/Scripts/4_World/DayZExpansion_Core/Classes/Prefab/ExpansionPrefabObject.c).
The AI must load its chamber/internal magazine from the starting stack.
`UnlimitedReload: 1` uses stock replenishment during AI reloads, so ammunition
remaining on a corpse can exceed the initial allowance. Initial loading,
capacity and sustained use remain runtime acceptance items.

### Coordinates and assignment

Coordinates are copied from
[cfgplayerspawnpoints.xml](https://github.com/BohemiaInteractive/DayZ-Central-Economy/blob/f220b2918b1d79a5055b1ee9be70c0ec1516e876/dayzOffline.nasdara/cfgplayerspawnpoints.xml)
at the same CE commit; SHA-256
`d6edf317ed1508385994a928f86df2da8d812a382a22af594d03cf6df907764d`.
The exact path is `playerspawnpoints/fresh/generator_posbubbles/group/pos`.
All 20 unique centers retain source order and X/Z precision as `[X, 0, Z]`.
The `hop` and `travel` lists duplicate `fresh` and are excluded.

At each center, the file contains `Nasdara_Civilian_P01`-`P20` and
`Nasdara_Tribesman_P01`-`P20`. Civilian loadouts cycle 01-04 (five uses each);
Tribesman loadouts cycle 01-06 (four uses each of 01/02, three each of 03-06).
The array interleaves Civilian and Tribesman at each successive center.
Native spread is 1-2 m for Civilians and 4-5 m for Tribesmen, allowing both
roles to coexist around the same anchor without adding replacement coordinates.

These are unverified spawn-area centers, not the engine's filtered player
spawn candidates. Stock player spawning samples grids around these centers
and rejects unsuitable candidates. Expansion height resolution and random
spread do not reproduce that filter or establish flat, outdoor, clear ground.
No coordinates have been geometrically accepted.

### Shared behavior and native limits

| Setting | Configured value |
| --- | --- |
| `NumberOfAI` / `NumberOfAIMax` | 1 / 0, exactly one AI per patrol |
| `Faction` | `Civilian` for both equipment roles |
| `Behaviour` / `Speed` / `UnderThreatSpeed` | `ROAMING_LOCAL` / `WALK` / `SPRINT` |
| `ThreatDistanceLimit`, `NoiseInvestigationDistanceLimit`, `MaxFlankingDistance` | 45.72 m (50 yards) each, also set globally |
| `EnableFlankingOutsideCombat` | 0 globally and per patrol |
| `MinDistRadius` / `MaxDistRadius` | 45.72 / 500 m activation distances |
| `DespawnRadius` / `DespawnTime` / `RespawnTime` | 600 m / 300 s / 1800 s |
| `Chance`, `CanBeLooted`, `UnlimitedReload` | 1 each |
| `Persist`, `CanBeTriggeredByAI`, `CanSpawnInContaminatedArea` | 0 each |
| Load-balancing caps, player counts 0-255 | Global 40; Civilian 20; Tribesman 20 |

The caps are maximum concurrent patrol counts, not a promise that all 40 spawn
at once: player proximity and other native spawning rules still apply.
`MaxDistRadius: 500` controls activation distance, not a movement leash.
Stock local roaming selects nearby destinations and can fall back to roaming
locations farther away; it may exceed the desired 500 m area. No new roaming
locations or boundary enforcement are implemented. Stock location settings
generated for the deployed world still require runtime inspection.
The 45.72 m detection/investigation/flanking settings likewise do not limit
travel to a wandering destination or replace stock combat/assistance logic.

Civilian scavenging is explicitly
`WEAPONS_FIREARMS | WEAPONS_MELEE | BANDAGES | CLOTHING | UPGRADE`.
Tribesman scavenging is
`WEAPONS_MELEE | BANDAGES | CLOTHING | UPGRADE`, excluding firearm replacement.
These allowlists intentionally omit launchers. Starting outfits and corpse
contents can change through scavenging; civilians can acquire firearms.

`PlayerFactions: ["West"]` supplies mutual friendship with stock Civilian
patrols. Existing persisted player groups can already have a faction; the
connect-time default does not guarantee reassignment of those groups.
Both `CanRecruitFriendly` and `CanRecruitGuards` are 0 so players cannot recruit
these individuals into larger patrols. Stock retaliation, aggression expiry,
and assistance to friendly patrols are preserved. This phase adds neither the
custom targeting rules discussed below nor Warlord/Western_Merc patrols.

### Configuration validation and later acceptance

Repo-local `dzcli` v0.0.5 was installed/checked before edits. The baseline
`dzcli validate expansion ai <server-root>` exited 1 because this server had
no AI configuration; its exact output is retained. Loadouts and patrol entries
were created with supported commands, each previewed with `--dry-run` and
compared with the actual written file. Full AI settings and patrol-root
settings/categories used direct edits because no matching mutations exist.

Two v0.0.5 limitations were observed and worked around:

- JSON output mode reports mutation success without writing. JSON was used for
  dry-run previews; text mode was used to apply the same mutation.
- Server-root discovery finds profiles but does not descend through
  `mpmissions/<mission>`. The final checks therefore validate both the server
  root and `mpmissions/dayzOffline.nasdara` explicitly, covering all 12 files.
  Loadout references were also checked independently against the ten files.

Final validators pass without warnings or failures. The static audit checks
JSON parsing, 10 loadouts, 40 single-AI definitions, all 45 inventory classes
against the pinned allowlist, exact decimal source coordinates, role-specific
cycling/spread/looting, quantities/chances/health, shared settings, and caps.
These results establish configuration consistency only. No tests were added
and no DayZ runtime verification was performed.

Deployment prerequisites are listed in the
[server README](README.md#expansion-ai-configuration). Before runtime acceptance,
install and load compatible stock CF, Dabs Framework, Expansion Core and
Expansion AI on the host/clients, deploy this partial mission overlay and
profile files, and retain the complete installed stock Nasdara mission.
The existing launchers still activate only CF and VPPAdminTools.

Later runtime acceptance remains pending:

- [ ] Check both spawn spreads for ground support, outdoor placement, collision
      clearance, correct elevation and a walking exit at the copied centers.
- [ ] Check every outfit and alternative for attachment fit and cargo capacity,
      including the largest tools and all optional supplies appearing together.
- [ ] Check all five firearm classes load from their loose ammo, fire and reload.
- [ ] Check initial ammo fractions and sustained unlimited reload, including
      corpse ammunition after replenishment.
- [ ] Check independent supply rolls, bottle liquid defaults, clothing upgrades,
      Civilian firearm pickup and Tribesman firearm retention; loot corpses.
- [ ] Check walking outside combat, sprinting under threat, native roaming
      destinations and travel beyond the desired area.
- [ ] Check activation at/above 45.72 m and within 500 m, 600 m despawn distance,
      timers, and caps with several player counts.
- [ ] Check sight/hearing/flanking around 45.72 m, player friendship,
      retaliation, friendly assistance, existing player-group factions and
      disabled recruitment.

## Faction and loadout design

### Confirmed gameplay policy

The requested setting uses Tribesmen, Civilians, Warlords, and `Western_Merc`
patrols. `Western_Merc` replaces the earlier NATO patrol concept and retains
NATO-style equipment. The following table describes intended behavior toward
other patrols and players. The current phase implements only the two friendly
populations through stock `Civilian`; the full four-role policy is future work.

| Acting patrol | Tribesmen | Civilians | Warlords | Western_Merc | Players |
| --- | --- | --- | --- | --- | --- |
| Tribesmen | Neutral* | Neutral* | Neutral* | Neutral* | Neutral* |
| Civilians | Neutral* | Neutral* | Neutral* | Neutral* | Neutral* |
| Warlords | Neutral* | Neutral* | Hostile | Hostile | Hostile |
| Western_Merc | Hostile | Hostile | Hostile | Hostile | Hostile |

*Neutral means no unprovoked engagement, with retaliation and assistance to
friendly patrols allowed. The user explicitly chose assistance beyond the
attacked patrol. Cross-patrol fighting is intended between Warlord groups and
between Western_Merc groups. Members of the same patrol remain a group; this
does not request infighting within a patrol.

Helping friendly patrols can pull neutral populations into nearby battles,
including fights between Warlord groups. Neutrality is therefore an initial
relationship, not immunity from combat. Assistance eligibility and aggression
expiry use stock behavior in this phase (120 seconds, guard 150 seconds) and
still require runtime checks.

### What existing Expansion factions provide

Research used public `release` commit
`0d26645ca31bc574bc3ebee1d4bc9192727f3c43`. The 13 inspected faction, patrol,
targeting, settings, and player integration files matched their counterparts
at `experimental` commit `14fd692bd7eb46df05853ea31ee3d481239a826c`.
This is source inspection; no game session or faction encounter was run.

| Role | Starting point | Required work or limitation |
| --- | --- | --- |
| Tribesmen | `Civilian` behavior with armed local loadouts | Implemented for this phase; exact restraint against later hostile factions would need the targeting work below. |
| Civilians | Same behavior as Tribesmen, separate civilian loadouts | Implemented with tool-only starting inventory; the approved looting policy permits later firearm pickup. |
| Warlords | Custom registered faction | Preserve neutrality with civilian populations while allowing hostility between different Warlord groups. |
| Western_Merc | `Raiders`, with Western military loadouts | Provides hostility between separate mercenary groups; exact neutral first-fire behavior needs the targeting work below. |

- `Civilian.IsFriendly` returns true for every faction. `East` and `West`
  accept their own faction and Civilian-derived factions. Consequently,
  `East` cannot provide the requested Warlord infighting.
- `Raiders` only exempts passive factions; it does not exempt ordinary
  `Civilian`. It fits Western_Merc's broad hostility, but needs modification
  to fit Warlords. Stock `Mercenaries` accepts its own faction, so it does not
  provide the requested fighting between Western_Merc patrols.
- A proposed Warlord faction would recognize Civilian-derived factions and
  passive factions, and omit a friendship exception for its own faction.
  Keep the same-group protection provided by Expansion. This design needs a
  registered script class, not just a new string in patrol JSON.

Sources: [Civilian](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/0d26645ca31bc574bc3ebee1d4bc9192727f3c43/DayZExpansion/AI/Scripts/3_Game/DayZExpansion_AI/Factions/eAIFactionCivilian.c),
[East](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/0d26645ca31bc574bc3ebee1d4bc9192727f3c43/DayZExpansion/AI/Scripts/3_Game/DayZExpansion_AI/Factions/eAIFactionEast.c),
[Raiders](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/0d26645ca31bc574bc3ebee1d4bc9192727f3c43/DayZExpansion/AI/Scripts/3_Game/DayZExpansion_AI/Factions/eAIFactionRaiders.c),
[Mercenaries](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/0d26645ca31bc574bc3ebee1d4bc9192727f3c43/DayZExpansion/AI/Scripts/3_Game/DayZExpansion_AI/Factions/eAIFactionMercenaries.c),
[custom faction registration guide](https://github.com/salutesh/DayZ-Expansion-Scripts/wiki/%5BModding%5D-Adding-a-new-Faction).

### Targeting and retaliation limits

The stock relationship system does not guarantee a neutral faction will wait
to be hit by a hostile faction. `PlayerIsEnemy` checks the target's faction
attitude toward the evaluating AI and also reacts to the other AI's cached
threat. Inference: Civilian versus Raiders can enter combat before Civilian
receives damage. Exact one-way restraint requires additional targeting rules;
the suggested faction mapping alone is an approximation.

The hit handler can alert friendly AI outside the victim's group.
`AddFriendlyAI` requires mutual faction friendship for those other groups and
can propagate hostility to AI already aware of the aggressor. This supports
the requested assistance, with runtime limits; it is not a guarantee every
friendly patrol across the map joins the fight. Decide whether standard mutual
friendship or a separate support policy governs custom neutral targeting.

Sources: [enemy selection](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/0d26645ca31bc574bc3ebee1d4bc9192727f3c43/DayZExpansion/AI/Scripts/4_World/DayZExpansion_AI/Entities/AI/eAIBase.c#L969),
[hit response](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/0d26645ca31bc574bc3ebee1d4bc9192727f3c43/DayZExpansion/AI/Scripts/4_World/DayZExpansion_AI/Entities/DayZPlayerImplement.c#L472),
[friendly assistance](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/0d26645ca31bc574bc3ebee1d4bc9192727f3c43/DayZExpansion/AI/Scripts/4_World/DayZExpansion_AI/Classes/Targets/eAITargetInformation.c#L388).

### Equipment and player relationships

`Faction`, patrol `Name`, and `Loadout` are separate fields. A populated
`Loadout` overrides the faction's default equipment. Tribesmen and Civilians
can share a behavior faction while using different equipment files. A patrol
named `Western_Merc_01` can use `Faction: Raiders` and a Western loadout; a
literal `Faction: Western_Merc` requires registering that faction first.

For a future civilian role intended to remain unarmed, a weapon-free loadout
also needs an allowlist without weapon flags or `LootingBehaviour: NONE`.
The approved current Civilian role instead starts without firearms and may
scavenge them. Default looting permits weapons, and roaming can default to `ALL`.
Unarmed AI may still use melee when hostile; fleeing-only behavior is not
established by omitting a gun. `Passive` and `Observers` are unsuitable for the
requested normal retaliation role.

The approved player assignment is `PlayerFactions: ["West"]`. Do not put every
player in `Civilian` if future Warlords exempt that faction. The `West` name
does not require NATO equipment or a NATO patrol role. Existing persistent
player groups may already have a faction and must be considered separately.

Sources: [patrol fields](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/0d26645ca31bc574bc3ebee1d4bc9192727f3c43/DayZExpansion/AI/Scripts/3_Game/DayZExpansion_AI/Settings/Patrols/ExpansionAIPatrolBase.c),
[patrol spawning and loadout fallback](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/0d26645ca31bc574bc3ebee1d4bc9192727f3c43/DayZExpansion/AI/Scripts/4_World/DayZExpansion_AI/Classes/Patrols/eAIDynamicPatrol.c#L599),
[looting options](https://github.com/salutesh/DayZ-Expansion-Scripts/wiki/%5BServer-Hosting%5D-How-to-create-AI-Patrols#lootingbehaviour),
[player faction assignment](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/0d26645ca31bc574bc3ebee1d4bc9192727f3c43/DayZExpansion/AI/Scripts/5_Mission/DayZExpansion_AI/Mission/MissionServer.c#L60).

### Badlands equipment research

Research on 2026-10-19 checked the Badlands 1.30 release notes, the earlier
1.29 Road to Badlands release, and official economy/script snapshots. Relevant
families include Kameez and Shalwar clothing, Pakol and Turban headwear,
Keffiyehs, linen and robe vests, chest rigs, OBR and FNO uniforms, desert gear,
SCR16/SCR17 rifles, Lee Enfield, Luger, MP18, and regional melee weapons.
The implemented subset is the ten approved loadouts above.

Verified class examples include `KameezShirt_Beige`, `MensShalwarPants_Brown`,
`PakolHat_Beige`, `RobeVest_Beige`, `FNOJacket`, `FNOPants`, `SCARL` (SCR16),
and `SCARH` (SCR17). The Robe is implemented as a vest layer. Akinaka uses
`Akinaka`, despite the release notes shortening the name to Akina.

The checked Nasdara economy at `f220b2918b1d79a5055b1ee9be70c0ec1516e876`
is not a complete engine class catalog: it omits `PlateCarrierVest_Desert`,
which is present in the earlier official Chernarus economy at
`b86c1b71cbad4b62c4ca004f6ec7c8d4bbaccead`. That research distinguishes an
engine class catalog from a mission economy catalog. The user subsequently
chose the pinned Nasdara economy as the strict allowlist for this phase, so
unlisted classes are excluded even when the engine supplies them. Item
presence does not demonstrate Expansion AI weapon handling or a working loadout.

Sources: [Badlands release notes](https://dayz.com/article/general/dayz-badlands-out-now/dayz-steam-patch-notes),
[Road to Badlands release](https://dayz.com/article/game-update/dayz-update-129-road-to-badlands),
[Nasdara item definitions](https://github.com/BohemiaInteractive/DayZ-Central-Economy/blob/f220b2918b1d79a5055b1ee9be70c0ec1516e876/dayzOffline.nasdara/db/types.xml),
[earlier Chernarus definitions](https://github.com/BohemiaInteractive/DayZ-Central-Economy/blob/b86c1b71cbad4b62c4ca004f6ec7c8d4bbaccead/dayzOffline.chernarusplus/db/types.xml),
[1.30 scripts](https://github.com/BohemiaInteractive/DayZ-Script-Diff/tree/e3dda95b2b9730bdc0dba931abb35304b484bbfc).

### Implementation work identified by research

- [ ] Future phase: decide whether to authorize a custom Warlord faction and
      exact neutral targeting; neither is included in this stock-only phase.
- [ ] Future phase: implement Western_Merc after its separate loadout and
      targeting policy are approved.
- [x] Configure West players, stock aggression expiry, and disabled recruitment
      for the two implemented populations; runtime behavior remains unchecked.
- [x] Define ten starting loadouts and explicit per-role looting policies.
      Civilian firearm pickup is deliberately allowed.
- [ ] Include encounter scenarios in the later automated runtime validation:
      each faction pair, player attacks, friendly assistance, separate hostile
      groups, same-group protection, and civilians near dropped weapons.

These tasks do not replace the six spawn-scanning steps below. They provide
the gameplay policy and runtime behavior required by the Step 6 generator.

## Prerequisite: prove the offline scanning environment

- [ ] Identify the local DayZ installation and installed Nasdara version/assets.
- [ ] Confirm compatible DayZ Editor, CF, and Dabs Framework versions and load
      Nasdara locally without a dedicated server.
- [ ] Establish a scanner entry point, local input/output paths, and a start
      action that does not require repeated script entry.
- [ ] Demonstrate terrain, surface identity, object collision, and navigation
      queries in that session before running a large batch.
- [ ] Demonstrate movement between scan areas and a reliable readiness check
      for streamed collision geometry. An empty object query alone is not
      evidence that an area is loaded and clear.
- [ ] Check the acceptance logic against automatically identifiable examples
      of blocked, elevated, interior, and usable ground positions. Verify that
      unavailable geometry or navigation produces an unresolved result.

If a required offline capability is unavailable, record the evidence and revise
the execution approach here before claiming that it can validate spawn points.

## Step 1: Generate thousands of candidates

Status: not started.

- [ ] Select and record a complete Nasdara mission source snapshot appropriate
      for the installed map. Record commit/release and file checksums.
- [ ] Parse event positions, player spawn areas, mapped economy objects, and
      relevant hazard/underground definitions.
- [ ] Derive search areas around settlements, military sites, roads, and other
      useful patrol areas. Record each area's source and intended role.
- [ ] Generate a configurable candidate grid around those areas, with enough
      alternatives to replace failures. Do not assume an event's exact spawn
      coordinate is safe or unoccupied.
- [ ] Apply preliminary exclusions, including configured event footprints,
      hazards, and user-defined restricted areas. Document the limits of any
      object exclusion estimated without complete geometry.
- [ ] Assign stable candidate IDs, remove duplicates, and order candidates
      deterministically for reproducible scans.

Output: candidate coordinates and area metadata, an area quota definition, and a
source manifest. All coordinates remain unverified at this stage.

## Step 2: Load and scan areas automatically

Status: not started; depends on the offline environment prerequisite.

- [ ] Group candidates into spatial scan sections to reduce world movement.
- [ ] Move the local viewpoint/character automatically and wait for the
      section's geometry readiness criteria.
- [ ] Use a bounded queue and small batches so the client remains responsive.
- [ ] Add timeouts, bounded retries, and explicit unresolved outcomes.
- [ ] Save progress after each section and support resuming without losing
      completed results or duplicating accepted locations.
- [ ] Reject incompatible checkpoints when source data, map/build, loaded
      object mods, or acceptance settings change.

Output: resumable progress and a measured result for each processed candidate.
The operator starts a run; movement between individual points is automatic.

## Step 3: Reject unsuitable positions automatically

Status: not started.

- [ ] Resolve supporting surfaces and elevation; reject unwanted object
      surfaces, elevated structures, and unsupported positions.
- [ ] Measure local slope and abrupt height changes across the required
      standing footprint. A proposed initial slope limit is 5-8 degrees;
      select and validate the actual threshold during implementation.
- [ ] Reject interiors and overhead roofs/structures using appropriate world
      queries, rather than relying on a single raycast or a building name.
- [ ] Test standing-character clearance against relevant collision geometry.
- [ ] Reject water and configured hazards/restricted areas.
- [ ] Sample appropriate walkable navigation and require a usable nearby exit.
      Reject navigation that depends on disallowed interiors, swimming,
      ladders, or other movements outside the agreed spawn policy.
- [ ] Record measured values and specific rejection reasons such as
      `too_steep`, `uneven_ground`, `roof_overhead`, `object_surface`,
      `body_blocked`, `water`, `no_walkable_exit`, or `geometry_unresolved`.

Output: accepted or rejected geometry/navigation results with evidence. Numeric
limits, road treatment, clearance volumes, and query methods remain to be
implemented and validated, not assumed correct from the plan.

## Step 4: Validate complete group placements

Status: not started.

- [ ] Define supported group sizes, formations, scale, and orientation policy.
- [ ] Calculate and check every possible initial member placement using the
      same acceptance rules as the anchor.
- [ ] If initial orientation is uncontrolled, validate a conservative envelope
      covering the possible formation positions.
- [ ] Reject the group placement when any required member position fails.
- [ ] Keep random spread at zero initially. Require additional surrounding
      area checks before enabling any nonzero spread.
- [ ] Record which group configurations each accepted anchor supports.

Output: accepted group anchors with checked member positions or a checked
formation envelope. Anchor acceptance alone is insufficient.

## Step 5: Automatically replace failures

Status: not started.

- [ ] Set a requested accepted count and distribution across search areas.
- [ ] Select replacements automatically when a candidate or group fails.
- [ ] Enforce configurable separation and deduplication between selected
      anchors while preserving the requested area distribution.
- [ ] Continue until each area's quota is met or its bounded search budget is
      exhausted. Expand a search only within configured limits.
- [ ] Report unmet quotas and their rejection patterns; do not silently lower
      acceptance thresholds to reach a target.

Output: a selected spawn inventory plus explicit per-area completion/shortfall
results. Rejected candidates require no individual human approval.

## Step 6: Export ready-to-use results

Status: not started.

- [ ] Export accepted `[X, Y, Z]` coordinates, stable names, area assignments,
      supported group configurations, and measurement evidence.
- [ ] Export rejection reasons, run summaries, manifests, and checkpoints.
- [ ] Record the map/build, loaded object mods/mapping, source snapshot, scanner
      version, acceptance settings, and scan time with each run.
- [ ] Build a patrol generator that consumes the accepted inventory and an
      explicit faction/loadout/behavior/load-balancing policy.
- [ ] Keep generated patrol definitions traceable to accepted spawn IDs and
      enforce the checked formation/spread constraints.
- [ ] Verify loadout references and run matching `dzcli` validation before
      describing generated Expansion configuration as ready for use.
- [ ] Define a separate automated validation stage for actual Expansion AI
      spawning and movement when an appropriate runtime environment is
      available. Offline geometry acceptance must not be reported as passing
      that stage.

Output: machine-readable spawn inventory and reports, followed by generated,
validated Expansion patrol configuration when its gameplay policy is defined.

## Completion criteria

- One local offline Nasdara session can process a batch without per-point
  inspection, teleporting, script entry, or approval by the operator.
- Every accepted group passes all required geometry and navigation checks.
- Interruptions can resume safely, and incompatible cached results are refused.
- Requested quotas are met or accompanied by explicit, actionable shortfalls.
- Exports are reproducible and traceable to measured evidence and map versions.
- Actual AI runtime verification is identified separately from offline checks.

## Next implementation work

1. Arrange separate deployment of the required stock mods and prepared files.
2. Complete the runtime acceptance checklist for this 40-patrol phase.
3. When scanner work is authorized, establish the offline environment/query
   prerequisite, define its schemas and implement a small end-to-end batch.
4. Use that batch's evidence to refine Steps 1-6 before scaling to hundreds.

For the current phase, role distribution and group size are fixed at 20 per
role and one AI each, with the approved native spreads. Future scanner decisions
include area quotas, formation policy, minimum separation, numerical acceptance
thresholds, map-version matching, collision readiness detection, and packaging.

Follow the repository `dzcli` skill when inspecting or changing supported DayZ
configuration: bootstrap the local tool, inspect current values, preview
mutations with `--dry-run`, preserve baseline failures, and run matching
validators after changes. The six scanner steps remain unimplemented; the
current patrol files were built directly from copied, unverified centers rather
than exported from an accepted scanner inventory.

## Maintaining this plan

Update this file in the same worktree as implementation changes and whenever
new evidence changes a capability, assumption, acceptance rule, dependency, or
workflow. Mark checklist items complete only after the corresponding work is
demonstrated. Record failed approaches and their practical implications when
they affect the remaining plan. Keep source links and verification limits next
to the decisions they support.

### Decision and discovery log

| Date | Decision or discovery | Effect |
| --- | --- | --- |
| 2026-10-19 | Nasdara selected as the implementation target. | Use Nasdara data and map geometry throughout. |
| 2026-10-19 | Individual manual spawn review is impractical. | Automate rejection, replacements, quotas, and reports. |
| 2026-10-19 | Offline Editor proposed as the scanning environment. | Prove query and streaming behavior before scaling. |
| 2026-10-19 | Economy XML and zero waypoint height cannot establish all placement requirements. | Require world geometry checks and measured evidence. |
| 2026-10-19 | Initial six-step plan recorded. | Implementation remains pending; update this document as work proceeds. |
| 2026-10-19 | Western_Merc replaces NATO; Warlord and Western_Merc patrols each fight other patrols of their own type. Neutral populations may help friendly patrols. | Use a custom Warlord faction and Raiders as the Western_Merc starting point; keep loadouts separate from faction identity. |
| 2026-10-19 | Stock targeting can react to another faction's hostility before receiving a hit. | Exact neutral first-fire restraint needs custom targeting work and later runtime validation. |
| 2026-10-19 | Badlands equipment researched against release notes and official economy/script snapshots. | Keep display names separate from class names; mission economy files are not complete engine catalogs. |
| 2026-10-19 | Approved a configuration-only phase with 10 loadouts and 40 single-AI patrols at the pinned 20 player spawn centers. | Copy exact X/Z coordinates, use Y=0 and native 1-2/4-5 m spreads; defer geometry acceptance and all custom mod work. |
| 2026-10-19 | Pinned Nasdara types.xml is the loadout allowlist, despite the broader engine catalog. | All 45 used inventory classes are present; exclude unlisted additions. |
| 2026-10-19 | Civilians may scavenge firearms; Tribesmen may upgrade clothing/melee but retain starting firearms. | Explicit looting allowlists supersede the earlier suggestion that all civilians remain weapon-free. |
| 2026-10-19 | Expansion quantities are capacity fractions; unlimited reload refills native ammunition. | Use 0.20/0.25/0.40 initial stacks and document possible extra ammunition on corpses. |
| 2026-10-19 | Native local roaming is accepted without a hard boundary. | 500 m is an activation radius/roaming aim, not an enforced travel limit. |
| 2026-10-19 | dzcli v0.0.5 JSON mutations do not write, and server-root discovery misses nested mission settings. | Apply in text mode after JSON dry runs, compare written files with previews, and validate both profile/server and mission roots. |
| 2026-10-19 | Configuration validators and the requested static audit pass. | Deployment and runtime acceptance remain pending; no server or game was launched. |

## References

- [Bohemia Nasdara economy mission](https://github.com/BohemiaInteractive/DayZ-Central-Economy/tree/master/dayzOffline.nasdara)
- [Nasdara player spawn generation configuration](https://github.com/BohemiaInteractive/DayZ-Central-Economy/blob/master/dayzOffline.nasdara/cfgplayerspawnpoints.xml)
- [DayZ Editor official Workshop page](https://steamcommunity.com/sharedfiles/filedetails/?id=2250764298)
- [Editor installed-world discovery](https://github.com/InclementDab/DayZ-Editor/blob/legacy/DayZEditor/Scripts/4_World/DayZEditor/gui/Dialogs/EditorMapSelectDialog.c)
- [Editor script execution](https://github.com/InclementDab/DayZ-Editor/blob/legacy/DayZEditor/Scripts/4_World/DayZEditor/classes/Commands/Hud/EditorScriptEditorCommand.c)
- [Bohemia navigation API](https://github.com/BohemiaInteractive/DayZ-Script-Diff/blob/main/scripts/3_game/ai/aiworld.c)
- [Expansion placement implementation](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/experimental/DayZExpansion/AI/Scripts/3_Game/DayZExpansion_AI/Settings/Patrols/ExpansionAIPatrolBase.c#L118)

Upstream links using branch names can change. Pin the versions actually used by
the scanner and record them in run manifests before relying on their behavior.
