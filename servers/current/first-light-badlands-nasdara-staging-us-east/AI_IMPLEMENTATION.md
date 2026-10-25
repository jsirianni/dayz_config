# Nasdara Expansion AI Implementation

<!-- merc-stack-current:start -->
## Current active configuration — Startup and accuracy update (2026-10-22)

This section is authoritative. Older sections below record historical decisions
where superseded here, including town patrols, previous caps, and player bonus gear.
The broader Warlord research and six-step spawn scanner plan are retained.

### Population and relationships

The 40 Civilian/Tribesman patrols and their balancing categories are removed.
Their loadout files remain unassigned. The persistent **assasin remains**
as the explicit exception to military-site Merc patrols.

| Role | Patrol cap | AI per patrol | Maximum living AI | AccuracyMin–Max |
| --- | ---: | ---: | ---: | --- |
| Regular Merc | 10 | 5 | 50 | 0.30–0.7225 |
| Spec Ops | 6 | 5 | 30 | 0.35–0.8075 |
| Juggernaut | 1 | 5 | 5 | 0.35–0.8075 |
| assasin | 1 | 1 | 1 | 0.7225–0.8075 |
| **Total** | **18** | | **86** | |

`Global.MaxPatrols` is **18**: Expansion counts patrols, not individual AI.
Category caps apply across player counts 0–255; unused capacity is not borrowed.
Mercs have `NumberOfAIMax: 0`. Totals: **205 definitions / 47 loadout files**.

All Merc variants use `East`, cooperate with other Mercs, and are hostile to players
assigned `West`. Existing equipment files and profile `AISettings.json` are unchanged.
The assasin retains Raiders, its remote anchor, singleton, persistence and equipment.

### Startup updates and accuracy reduction (2026-10-22)

The game-server SteamCMD command retains `app_update 223350` and removes its
`validate` flag. Routine startups continue checking for and installing updates,
without forcing restoration of modified stock files on every start. Workshop
validation is unchanged. Actual game updates can still replace shipped mission
files; review and reapply the custom mission overlay after an update if needed.

Every patrol's `AccuracyMax` is reduced by 15% relative to its previous value
(`old maximum * 0.85`): 0.85 becomes 0.7225; 0.95 becomes 0.8075.
Merc minimums remain unchanged. The assassin's minimum also falls by 15%, from
0.85 to 0.7225, as agreed, so it stays below its new 0.8075 maximum.
These are gameplay settings, not measured hit percentages.

### Object spawning and behavior

All 51 classes / 460 officially listed placements remain in use; provenance is in
`validation/merc-object-classes.csv`. Object profiles have empty waypoints and
`Persist: 0`.
Each class has four ordered profiles. Chance is conditional on earlier failures.

| Profile order | Conditional Chance | Exact ratio |
| --- | ---: | --- |
| Juggernaut | 0.029411764705882353 | 1/34 |
| SpecOps | 0.18181818181818182 | 2/11 |
| BoltAction | 0.046296296296296294 | 5/108 |
| Regular | 0.33980582524271846 | 35/103 |

First-success selection preserves **50% eligibility** and nominal **10 ordinary :
6 Spec Ops : 1 Juggernaut** weighting. Ordinary kits retain their 12.5%/87.5%
BoltAction/Regular split and uniform original 16-kit weighting.
Eligibility/profile selection happens at object initialization; respawns retain
the profile. Caps are maxima, not guaranteed populations. A full category does not
make its objects switch to another profile.

Preserve local roaming, JOG/SPRINT, threat/flanking 200 m, noise investigation 100 m,
outside-combat flanking, activation 250–1000 m, despawn 1200 m / 300 s, respawn
1800 s, spread 40–80 m, unlimited reload, lootable corpses, no AI triggering and
no contaminated-area spawning. Full firearm/melee/bandage/clothing/upgrade scavenging
also applies to new variants, so equipment can change. Roaming can leave military
sites; generated placement does not establish safe terrain.

### Special equipment

`Nasdara_Western_Merc_SpecOps`: guaranteed suppressed `M4A1_Black`,
`M4_RISHndgrd_Black` / `M4_MPBttstck`, powered `M68Optic`, full attached
`Mag_STANAG_30Rnd` plus two full spares (90 magazine rounds).
Wear `TacticalShirt_Black` / `CargoPants_Black`, black military boots, tactical
gloves, plate carrier/pouches, `AssaultBag_Black`, and `BalaclavaMask_Black`.
`Mich2001Helmet` is the approved non-black exception, with powered `NVGoggles`.

`Nasdara_Western_Merc_Juggernaut`: standard `AKM` with iron sights,
`AK_RailHndgrd_Black` / `AK_PlasticBttstck_Black`, full attached
`Mag_AKM_Drum75Rnd` plus two full spares (225 magazine rounds).
Every squad member wears `Chainmail` / `Chainmail_Leggings`, `GreatHelm`, black
military boots/gloves, plate carrier/pouches and assault backpack. No suppressor,
optic, NVG, face covering or coif. The selected medieval Great Helm replaces the
ballistic-helmet proposal; it is not the strongest ballistic helmet.

Both kits start pristine, with one full bandage and a full native-water canteen.
No other medicine, food, radio, rangefinder, knife, flashlight or weapon light.
Required batteries are full. Explicit accuracy is 0.35–0.8075 and sniper-prone
threshold 0. Accuracy values are gameplay settings, not hit percentages.
Unlimited reload can increase later corpse ammunition; scavenging can introduce
replacement equipment or lights. These restrictions describe starting equipment.

Bind spare magazines and bandage to backpack cargo; canteen to shirt cargo.
Two 3x3 spare drums fit side by side in the 6x6 backpack, leaving room for the
bandage. The 2x2 canteen fits the 4x3 chainmail shirt. Native inventory placement
and firearm readiness still require runtime acceptance.

### Player combat kit — fresh characters only

Remove all twelve 5% bonus categories/helpers from `init.c`. Preserve stock random
regional clothing/sandals, soda/chemlight, figs, stock health randomization and
starting water 1000 / energy 800.

- Guaranteed pristine `PlateCarrierVest` / `PlateCarrierPouches` and
  `GorkaHelmet` / `GorkaHelmetVisor`.
- Exactly two full `BandageDressing` rolls total, one `SalineBag`, one
  `StartKitIV`, one `Epinephrine` and two `Morphine`. Saline/IV remain separate.
- Equal SKS/Mosin selection, unloaded rifle on shoulder; four full 20-round
  stacks (80 rounds) of matching standard `Ammo_762x39` / `Ammo_762x54`.
- Independent 50% `PUScopeOptic` in cargo, never preattached.
- `Glock19` with full attached 15-round `Mag_Glock_15Rnd` and chambered round;
  no spare pistol magazine or loose 9 mm.
- One unprimed `RGD5Grenade` attached to carrier. No backpack or old bonus gear.

Use deliberate clothing/pouch cargo bindings, including space for the scope.
Native packing audit: shirt 9/9 cells, pants 10/12 cells and pouches 15/15 cells
with the scope (34/36 total). The soda is explicitly rotated in the pants.
See `validation/player-combat-kit-audit.json` for all four rifle/scope cases.
All 14 shirt and 13 Shalwar variants use these same native cargo dimensions.
Log creation failures; cargo fallback is inventory, hands, then surface placement.
Parent-dependent attachments never become standalone substitute gear.
Only `StartingEquipSetup` supplies fresh characters; reconnects retain inventories.

### Validation and deployment

Historical stack evidence: `validation/merc-stack-06.txt` and `merc-stack-06-audit.json`.
The startup/accuracy update passed baseline and final `dzcli validate expansion ai`
checks against both the server root and `mpmissions/dayzOffline.nasdara`. Every
supported accuracy edit was previewed with `--dry-run` and applied through dzcli.
The static audit confirms all 205 maximums fell by exactly 15%, all ranges remain
valid, only the agreed assassin minimum changed, and other patrol fields are
unchanged. Launcher comparison confirms game updates and Workshop validation
remain enabled. These checks do not establish runtime behavior.
Preview/apply supported mutations with dzcli. Category maps and profile ordering
require direct edits because dzcli has no sufficient interface; initializer code
also requires direct edits. Audit original loadout hashes, assassin non-accuracy fields and
profile settings, counts, item allowlist, probabilities, classes, caps and behavior.

For deployment, stop the server, back up config/profile, and copy mission patrol
settings and the complete profile loadout directory into the existing installation.
Retain persistence so the assassin stays intact.
Also back up and replace the deployed mission `init.c`; no persistence wipe is needed.
Deploy the updated `dayz-service.bat` alongside the mission overlay. The launcher
updates the game server without forced SteamCMD validation. This repository change
does not deploy or launch DayZ. Later acceptance: group sizes, **86-AI ceiling**,
placement, faction behavior, accuracy, lifecycle timing and scavenging.
Check special equipment, powered optics/NVGs, supply counts and full drum storage.
Check both player rifles and scope outcomes, exact ammo/medicine, pistol readiness,
unprimed grenade and stock basics; check reconnect inventories remain unchanged.
Configuration validation is not Enforce compilation or in-game verification.

<!-- merc-stack-current:end -->

Last updated: 2026-10-22

## Promotion to a public server (2026-10-20)

The former staging server is promoted to full-time public use with hostname
`First Light | Badlands Nasdara | Expansion AI | us-east1` and no join password.
Its repository/deployment directory remains
`servers/first-light-badlands-nasdara-staging-us-east`. A customized stock Nasdara
`init.c` adds twelve independent 5% player spawn-gear categories; see
[the player gear and deployment details](README.md#public-promotion-and-player-spawn-gear-2026-10-20).
All AI patrols, loadouts, factions, 75% town-patrol chance, population caps and
the six-step scanner plan are unchanged. Historical staging references below
retain the context of their original changes. No deployment or server launch
is performed by this change.

## Staging relocation (historical)

This plan and the complete AI configuration now belong to
`servers/first-light-badlands-nasdara-staging-us-east`. They were relocated
from PR #58 head `14c8047` after rebasing onto the staging baseline. All 42
loadouts, profile AI settings, and the 40-patrol mission file retain their
original contents. Production Nasdara is restored to main `5c6306f`.

The six AI validation reports under `validation/` are preserved historical
records, including their original production paths. Current staging validation
is recorded in `validation/staging-ai-relocation.txt` and
`validation/staging-ai-relocation-audit.json`. Equipment policy, patrol assignments, combat tiers,
faction implementation, and scanner scope are unchanged by the relocation.

The staging launcher installs, synchronizes, and loads CF, VPPAdminTools,
Dabs Framework, Expansion Core, and Expansion AI in that order. PR #58 was
subsequently merged to main. The operator reports that the staging server is
deployed on a separate virtual machine, starts successfully, and shows the
correct mod list in DZSA. This is an operator report of the prior deployment,
not runtime verification of the object-patrol changes below. Profile/mission
overlays still require the manual copies described in README.

## Civilian and Tribesman 75% spawn chance (2026-10-20)

Set `Chance: 0.75` on all **20 Civilian and 20 Tribesman definitions**. Both
roles retain one AI per patrol, disabled scavenging, lootable corpses and the
existing faction relationships. Population caps still permit 51 patrols / 61
living AI; 75% eligibility does not guarantee that exactly 75% are active.

The stock patrol manager rolls `Chance` when initializing a patrol definition.
This is not a new roll each time a player enters the activation radius.
Successful patrols still obey distance, capacity and lifecycle conditions.
See [patrol initialization](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/0d26645ca31bc574bc3ebee1d4bc9192727f3c43/DayZExpansion/AI/Scripts/4_World/DayZExpansion_AI/Classes/Patrols/PatrolManager.c#L25).

The requested switch to Police was conditional on preserving gameplay.
**Stock Expansion has no Police faction.** `PoliceLoadout` is the default kit
of stock `Guards`, which sets `m_IsGuard: true` and accepts guard/passive
factions; Civilian accepts all factions. Switching to Guards would change
friendship and guard behavior. Tribesmen therefore remain `Faction: Civilian`,
with their existing Tribesman kits. No custom faction or loadout is added.
Sources: [Guards](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/0d26645ca31bc574bc3ebee1d4bc9192727f3c43/DayZExpansion/AI/Scripts/3_Game/DayZExpansion_AI/Factions/eAIFactionGuards.c),
[Civilian](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/0d26645ca31bc574bc3ebee1d4bc9192727f3c43/DayZExpansion/AI/Scripts/3_Game/DayZExpansion_AI/Factions/eAIFactionCivilian.c), and
[stock faction sources](https://github.com/salutesh/DayZ-Expansion-Scripts/tree/0d26645ca31bc574bc3ebee1d4bc9192727f3c43/DayZExpansion/AI/Scripts/3_Game/DayZExpansion_AI/Factions).

Only the 40 `Chance` fields change in this follow-up. Every supported mutation
was previewed with dzcli and matched to the applied output. Baseline/final
validation passes for both roots with identical output. Records:
[server baseline](validation/civilian-tribesman-chance-baseline.txt),
[mission baseline](validation/civilian-tribesman-chance-mission-baseline.txt),
[server final](validation/civilian-tribesman-chance-final.txt),
[mission final](validation/civilian-tribesman-chance-mission-final.txt), and
[static audit](validation/civilian-tribesman-chance-audit.json).

Deploy the mission patrol file with staging stopped and restart through the
existing launcher. Retain profile settings, loadouts and persistence. Later
staging acceptance should inspect eligibility over repeated initialization
cycles and confirm player friendship, single-AI groups and disabled scavenging.
No tests, server launch or deployment are performed; these are configuration
and source checks, not in-game verification.

## Restore single-AI Civilian and Tribesman patrols (2026-10-20)

Reduce `NumberOfAI` from 2 to **1** on all **20 Civilian and 20 Tribesman
patrol definitions**; retain `NumberOfAIMax: 0` for a fixed one AI per patrol.
Both roles retain `LootingBehaviour: NONE` and `CanBeLooted: 1`. Category caps
stay at 20 patrols each, while `Global.MaxPatrols` returns from 101 to **61**
across player counts 0-255. This supersedes the earlier pair-size increase.

| Category | Maximum patrols | AI per patrol | Maximum living AI |
| --- | ---: | ---: | ---: |
| Civilian | 20 | 1 | 20 |
| Tribesman | 20 | 1 | 20 |
| Western_Merc | 10 | 2 | 20 |
| Assasin | 1 | 1 | 1 |
| Total | **51** | | **61** |

The follow-up changes only 40 `NumberOfAI` fields and the global cap. All
other settings, 143 patrol definitions and 45 loadouts are preserved.
All 40 supported mutations were previewed with dzcli and matched to the
applied output. The global cap required a direct edit because dzcli v0.0.5
has no load-balancing-category mutation. Baseline/final server and mission
validation pass with matching output. Records:
[server baseline](validation/civilian-tribesman-singletons-baseline.txt),
[mission baseline](validation/civilian-tribesman-singletons-mission-baseline.txt),
[server final](validation/civilian-tribesman-singletons-final.txt),
[mission final](validation/civilian-tribesman-singletons-mission-final.txt), and
[static audit](validation/civilian-tribesman-singletons-audit.json).

Deploy only the mission `AIPatrolSettings.json` with staging stopped, then
restart using the existing launcher. Retain profiles, loadouts and persistence.
Later staging acceptance must confirm one AI per Civilian/Tribesman patrol,
no scavenging, lootable corpses and the 61-AI population ceiling. No server
launch, deployment or tests are performed; these are configuration checks.

## Disable Civilian and Tribesman scavenging (2026-10-20)

Set `LootingBehaviour: "NONE"` on all **20 Civilian and 20 Tribesman patrol
definitions** so they stop collecting town loot. This supersedes the original
allowlists, including Civilian firearm pickup and both roles' clothing upgrades.
`CanBeLooted: 1` remains: players can still loot their corpses. Merc scavenging
and the assasin's `BANDAGES` setting retain their current values.

The scavenging phase changed only those 40 `LootingBehaviour` fields and
preserved the then-current pair sizes and 101-AI ceiling. The follow-up above
restores one AI per patrol and the 61-AI ceiling. All loadouts, factions,
coordinates, spawn rules, combat and profile settings are preserved. The configuration
still contains 143 patrol definitions; Warlord patrols remain deferred.

All 40 mutations used supported dzcli commands with reviewed `--dry-run`
previews matching applied output. Baseline/final AI validation passes for both
server and mission roots, with identical output. See
[server baseline](validation/civilian-tribesman-no-looting-baseline.txt),
[mission baseline](validation/civilian-tribesman-no-looting-mission-baseline.txt),
[server final](validation/civilian-tribesman-no-looting-final.txt),
[mission final](validation/civilian-tribesman-no-looting-mission-final.txt), and
[static audit](validation/civilian-tribesman-no-looting-audit.json).
The [Expansion patrol guide](https://github.com/salutesh/DayZ-Expansion-Scripts/wiki/%5BServer-Hosting%5D-How-to-create-AI-Patrols#lootingbehaviour)
documents `NONE` separately from corpse lootability.

Deploy only the updated mission `AIPatrolSettings.json` with staging stopped,
then restart using the existing launcher; preserve profiles, loadouts and
persistence. Civilian/Tribesman groups remain nonpersistent. Later staging
acceptance must confirm they ignore dropped weapons, bandages and clothing,
retain their starting kit, and have lootable corpses. These are configuration
checks; no tests, server launch or deployment are performed.

## Civilian and Tribesman pair update (2026-10-20, historical)

This section records the earlier pair-size increase. The current single-AI
update above supersedes its group sizes and 101-AI ceiling; its audit remains
a historical record.

All **20 Civilian and 20 Tribesman patrol definitions** now use
`NumberOfAI: 2`, with `NumberOfAIMax: 0` retained for a fixed two AI per patrol.
Their category caps remain 20 patrols each. Raise `Global.MaxPatrols` from 61
to **101** across player counts 0-255. Role caps enforce the living-AI ceiling;
Expansion's global setting itself counts patrols.

| Category | Maximum patrols | AI per patrol | Maximum living AI |
| --- | ---: | ---: | ---: |
| Civilian | 20 | 2 | 40 |
| Tribesman | 20 | 2 | 40 |
| Western_Merc | 10 | 2 | 20 |
| Assasin | 1 | 1 | 1 |
| Total | **51** | | **101** |

The assassin retains its newly selected remote mausoleum anchor, persistence
and other settings. Civilian/Tribesman coordinates, native spreads, loadouts,
factions, recruitment restrictions and behavior are unchanged. Existing
143 patrol definitions and 45 loadouts remain; Warlord spawning is deferred.

Each of the 40 supported patrol mutations was previewed with `dzcli --dry-run`
and its result matched to the intended `NumberOfAI` change before applying.
The global category setting was edited directly because dzcli v0.0.5 does not
expose load-balancing-category mutations. Baseline and final server/mission
AI validation pass with identical output. See
[server baseline](validation/civilian-tribesman-pairs-baseline.txt),
[mission baseline](validation/civilian-tribesman-pairs-mission-baseline.txt),
[server final](validation/civilian-tribesman-pairs-final.txt),
[mission final](validation/civilian-tribesman-pairs-mission-final.txt), and
[static audit](validation/civilian-tribesman-pairs-audit.json).

Deploy the updated mission `AIPatrolSettings.json` with staging stopped, then
restart using the existing launcher; retain all profile/loadout/persistence
data. Civilian and Tribesman patrols remain nonpersistent and spawn fresh
under their existing activation rules. The living persistent assassin still
restores its saved position until a fresh spawn. No deployment is performed.
Later staging checks must confirm both members spawn with valid ground and
formation clearance, group cooperation, the 20/20 category limits, the 101-AI
ceiling, and host performance. These configuration checks are not in-game
verification; no tests, scripts, server launch or launcher changes are added.

## assasin spawn relocation (2026-10-20)

Replace the original 460 military-object starting waypoints with **one
uncommon-object anchor**, using the user's definition of an uncommon map
object/building. The spawn-relocation phase changed only
`assasin.Waypoints`. The subsequent pair update above changes 40 group sizes
and the global cap; all loadouts and profile settings retain their prior values.

| Selection | Value |
| --- | --- |
| Official class | `Land_Mausoleum_Small1_Dam1` |
| Class placements listed in official CE file | 2 |
| Selected source group index (one-based) | 13009 |
| Source X / Y / Z | `12170.946289 / 168.229233 / 2173.485107` |
| Configured waypoint | `[12170.946289, 0, 2173.485107]` |
| Nearest Merc object | `Land_Mil_Guardhouse2` at X `10866.722656`, Z `3305.604248` |
| Horizontal separation between anchors | **1727.05 m** |

Selection evaluated **324 listed placements across 138 classes**, considering
classes with one through five entries in the pinned official `mapgrouppos.xml`.
For each candidate, calculate the horizontal distance to the nearest of all
**460 Merc object placements**. The selected placement has the greatest such
distance in that candidate set. The other mausoleum of this class is only
83.47 m from a Merc object and is excluded. Counts describe CE placement
entries, not a census of every terrain object.

Coordinates and precision come directly from CE commit
`f220b2918b1d79a5055b1ee9be70c0ec1516e876`, with placement-file SHA-256
`4c1c1ba3c58371866805ac9b9cd7f00363529d022f436ad859a282e3d552f05e`.
See the [official placement file](https://github.com/BohemiaInteractive/DayZ-Central-Economy/blob/f220b2918b1d79a5055b1ee9be70c0ec1516e876/dayzOffline.nasdara/mapgrouppos.xml),
[current spawn manifest](validation/assasin-remote-spawn-points.csv), and
[selection/configuration audit](validation/assasin-remote-spawn-audit.json).
The earlier `assasin-spawn-points.csv` is retained as the historical 460-point
pool and the Merc-placement comparison record.

The patrol remains coordinate-based with `ObjectClassName: ""` and `Persist: 1`.
One waypoint means there is no choice among start anchors; existing 40-80 m
native spread still varies the initial position. Y=0 delegates elevation to
Expansion. This does not verify slope, outdoor clearance or ground placement.
Subtracting both patrols' maximum 80 m initial spreads gives about 1567 m of
geometric separation, subject to native placement behavior. Roaming can later
bring the assassin and Mercs together; separation is not a roaming boundary.

For deployment, stop staging and replace only the mission
`mpmissions/dayzOffline.nasdara/expansion/settings/AIPatrolSettings.json`, then
restart with the existing launcher. Retain profile settings, all loadouts and
persistence. **A living persisted assasin restores its old saved position; a
restart alone does not relocate it.** The new anchor applies to its next fresh
spawn after death/removal. No persistence reset or deployment is performed here.
The existing 3000 m minimum player distance still gates spawning/restoration.

Validation records: [server baseline](validation/assasin-remote-spawn-baseline.txt),
[mission baseline](validation/assasin-remote-spawn-mission-baseline.txt),
[server final](validation/assasin-remote-spawn-final.txt), and
[mission final](validation/assasin-remote-spawn-mission-final.txt).
Both final validators pass with output matching baseline; the static audit
confirms the waypoint-only change and source/distance correspondence. These
are configuration checks. Later staging checks must verify a fresh spawn near
this anchor, terrain clearance/elevation, persistence, and subsequent roaming.
No server launch, deployment, tests, launcher edits or custom scripts are added.

## Player relationship update (2026-10-20)

The current mapping supersedes the earlier West alliance for Tribesmen and
future Warlords. Change only `Faction` from `West` to `Civilian` on the **20
existing Tribesman patrol definitions**. Players remain `West`; 20 Civilian
patrols remain `Civilian`; all 102 Merc object definitions remain `East`;
`assasin` remains `Raiders`. Future Warlord patrols must use stock
`Mercenaries`; their 13 existing loadouts still have no patrol assignments.

| Role | Stock Expansion faction | Baseline encounters |
| --- | --- | --- |
| Players | `West` | Friendly to Civilians/Tribesmen; enemies of Mercs and future Warlords |
| Civilians | `Civilian` | Initially friendly to players, Tribesmen and Mercs; future Warlords can attack them |
| Tribesmen | `Civilian` | Initially friendly to players, Civilians and Mercs; future Warlords can attack them |
| Warlords (future patrols) | `Mercenaries` | Hostile to players, Mercs, Civilians and Tribesmen; cooperate with other Warlord patrols |
| Western_Merc | `East` | Hostile to players and future Warlords; initially friendly to Civilians/Tribesmen; cooperate with other Merc patrols |
| assasin | `Raiders` | Hostile to players and every implemented AI faction |

Stock retaliation and friendly assistance remain enabled. Friendly starting
relationships do not prevent later combat. This replaces the earlier plan
for Tribesmen and Warlords to cooperate. Warlords and Mercs are enemies of
each other and players; no custom faction or targeting code is added.

The player-relationship phase preserved **143 patrol definitions / 45 loadout
files**, loadout assignments, coordinates, behavior, accuracy and the then-current
61-living-AI ceiling. The pair phase raised it to 101; the latest reduction restores 61. The spawn
relocation changes the assasin anchor. Warlord spawning remains future work.
No tests, server launch, or deployment are performed.

Configuration records:

- [Server baseline](validation/player-factions-baseline.txt) and
  [mission baseline](validation/player-factions-mission-baseline.txt).
- [Server final](validation/player-factions-final.txt) and
  [mission final](validation/player-factions-mission-final.txt).
- [Static audit](validation/player-factions-audit.json).

Both final `dzcli` validators pass, with output matching their baselines.
The static audit confirms exactly 20 changed `Faction` fields, 40 Civilian / 102
East / one Raiders patrol definitions, no Warlord patrols, and preservation of
every other patrol field, all 45 loadouts, profile settings, and population
caps. These configuration checks do not establish in-game behavior.

For deployment, stop staging and replace only the mission
`mpmissions/dayzOffline.nasdara/expansion/settings/AIPatrolSettings.json`.
Retain existing profile `AISettings.json` with `PlayerFactions: ["West"]`,
loadouts, and persistence, then restart using the existing launcher. Existing
persistent player groups can retain an older faction and need separate
attention if their observed relationship differs; this update resets no data.
Later acceptance must check player friendship with both Civilian-faction
roles, player/Merc hostility, separate Merc cooperation, retaliation/assistance,
and unchanged assasin hostility. Warlord/Merc and Warlord/player hostility,
Warlord cooperation, and Warlord attacks on Civilians/Tribesmen require future
Warlord patrol implementation.

## assasin patrol update (2026-10-20)

Add one single-AI patrol named exactly `assasin`, with loadout
`Nasdara_Assasin.json` and category `Assasin`. It uses stock `Raiders`, hostile
to players and every implemented AI faction, including Civilians. The assasin
phase preserved the then-existing West/East alliances and 142 patrol definitions;
the current player relationship update above revises the Tribesman faction.
Warlord patrols remain future work.

The configuration now contains **143 patrol definitions / 45 loadout files**.
Load-balancing caps apply across player counts 0-255:

| Category | Maximum patrols | AI per patrol | Maximum living AI |
| --- | ---: | ---: | ---: |
| Civilian | 20 | 1 | 20 |
| Tribesman | 20 | 1 | 20 |
| Western_Merc | 10 | 2 | 20 |
| Assasin | 1 | 1 | 1 |

`Global.MaxPatrols` is now **61** after the size reduction. Category caps permit **51 active
patrols / 61 living AI**. Existing categories keep their previous patrol limits;
unused category capacity is not borrowed. Spawn rules can keep actual counts
below these ceilings.

### Fixed equipment

All 19 inventory items are guaranteed and pristine. The in-game VSD is class
`SVD`: one full `Mag_SVD_10Rnd` is attached, with one full spare magazine
(20 rounds initially). The spare supplies compatible ammunition for
`UnlimitedReload: 1`. Its equipment includes:

- `AK_Suppressor` in `weaponMuzzleAK`, `PSO6Optic` with a full `Battery9V`,
  and `GhillieAtt_Desert` in `weaponWrap`.
- `GhillieSuit_Desert` and `GhillieHood_Desert`, with USMC desert top/pants,
  beige boots/gloves, `TacticalGoggles`, and a beige balaclava.
- A rangefinder with a full battery, one full bandage roll, and a full-water
  `Canteen`.

`LootingBehaviour: BANDAGES` permits bandage pickup while preserving the
starting firearm and outfit. Unlimited reload can increase corpse ammunition
above the initial 20-round allocation. The existing 44 equipment/selector
files are unchanged.

### Spawn anchor and roaming

The single patrol now has **one waypoint**, copied from the remote official
`Land_Mausoleum_Small1_Dam1` placement at `[12170.946289, 0, 2173.485107]`.
The [spawn relocation](#assasin-spawn-relocation-2026-10-20) above records
selection criteria, source precision, 1727.05 m nearest-Merc separation,
validation and deployment implications. The former 460 military-object
waypoints are preserved only in the historical manifest/audit.

`UseRandomWaypointAsStartPoint: 1` is retained; with one waypoint it always
selects this anchor on fresh initialization, including after death/respawn.
The start must qualify for the 3000-25000 m player activation ring; otherwise
spawning may wait. `ObjectClassName` is empty; existing Merc object spawners
are unchanged. Waypoint Y=0 delegates elevation to Expansion, and native
40-80 m spread does not establish safe, flat, outdoor, unobstructed placement.

`ROAMING` selects map named locations after spawning. It does not provide an
omniscient hunter that always knows player positions. The 25 km activation
maximum and despawn radius aim to keep this patrol active anywhere on Nasdara
while players are online. `Persist: 1` preserves a living patrol across server
restarts, including its position and equipment; restoration does not reroll
its initial spawn. Restoring the saved position also obeys the 3000 m minimum;
a nearby player can delay restoration. Runtime persistence and lifecycle
behavior remain unchecked.

### Patrol settings

| Setting | Value |
| --- | --- |
| `Name` / `Faction` / `LoadBalancingCategory` | `assasin` / `Raiders` / `Assasin` |
| `NumberOfAI` / `NumberOfAIMax` | 1 / 0 |
| `Behaviour` / `Speed` / `UnderThreatSpeed` | `ROAMING` / `JOG` / `SPRINT` |
| `AccuracyMin` / `AccuracyMax` | 0.85 / 0.95 |
| `ThreatDistanceLimit` / `NoiseInvestigationDistanceLimit` | 1000 / 1000 m |
| `MaxFlankingDistance` / `EnableFlankingOutsideCombat` | 1000 m / 1 |
| `SniperProneDistanceThreshold` | 600 m |
| `MinDistRadius` | 3000 m |
| `MaxDistRadius` / `DespawnRadius` | 25000 / 25000 m |
| `MinSpreadRadius` / `MaxSpreadRadius` | 40 / 80 m |
| `DespawnTime` / `RespawnTime` | 300 / 1800 seconds |
| `CanBeLooted` / `UnlimitedReload` / `Persist` | 1 / 1 / 1 |
| `Chance` / `CanBeTriggeredByAI` / `CanSpawnInContaminatedArea` | 1 / 0 / 0 |
| `LootingBehaviour` | `BANDAGES` |
| `DamageMultiplier` / `DamageReceivedMultiplier` | 1 / 1 (explicit) |
| `HeadshotResistance` / `ShoryukenChance` | 0 / 0 |

Accuracy values are gameplay settings, not measured hit percentages.
Detection at 1000 m remains subject to visibility and stock target evaluation.
The PSO6's 1000 m maximum zeroing meets Expansion's optic qualification for
sniper-prone shooting; facing the target and no recent hit are also required.
The 600 m threshold does not guarantee a prone shot in every encounter.

### Configuration records and later acceptance

- [Server baseline](validation/assasin-baseline.txt) and
  [mission baseline](validation/assasin-mission-baseline.txt).
- [Server final](validation/assasin-final.txt) and
  [mission final](validation/assasin-mission-final.txt).
- [Static audit](validation/assasin-audit.json).

The original assasin-phase final validators passed, including all 143 patrol
definitions. Its historical audit covers 143/45 counts, the 61-AI ceiling,
category isolation, the former 460-source pool, random start/persistence
settings, inventory compatibility/quantities, and preservation of every
existing patrol and loadout. These are configuration checks, not in-game checks.
No tests, custom scripts/mods, launcher changes, server launch, or deployment
are part of this update.

Later staging acceptance must check:

- [ ] Outdoor ground support, clearance/elevation and 40-80 m spread around
      the selected mausoleum anchor after death and fresh initialization.
- [ ] Exactly one living assasin, 61-AI total ceiling, existing category limits,
      and expected 3000-25000 m activation, map roaming, and despawn/respawn behavior.
- [ ] Hostility toward every implemented faction and players, with the current
      baseline relationships between other patrol roles retained.
- [ ] VSD/PSO6/suppressor/wrap, full ghillie outfit, battery-powered equipment,
      inventory capacity, bandage-only scavenging, and lootable corpses.
- [ ] Unlimited reload, long-range detection, 600 m sniper-prone eligibility,
      flanking, jogging outside combat, and sprinting under threat.
- [ ] Living-patrol position/equipment persistence across restart and a fresh
      start at the selected anchor after death/respawn, without duplicates.

For deployment, stop staging and copy the updated mission
`AIPatrolSettings.json` and new `Nasdara_Assasin.json` into their existing
mission/profile destinations. Retain existing loadouts, profile data, and
persistence, then restart with the existing launcher.

## Faction alliance update (2026-10-20, historical)

This section records the earlier alliance phase. The current
[player relationship update](#player-relationship-update-2026-10-20)
supersedes its West assignments for Tribesmen and future Warlords; its
validation reports remain historical records.

The earlier alliance phase changed only `Faction` on **122 existing patrol definitions**:
20 Tribesman patrols use `West`, and 102 Western_Merc object definitions use
`East`. The 20 Civilian patrols remain `Civilian`. Players remain assigned
`West`. That phase planned future Warlord patrols as `West`; the user's Taliban reference
means this existing Warlord role, whose 13 loadouts still have no patrols.

West patrols cooperate across groups and fight East. Merc patrols cooperate
across groups. Both sides accept Civilians initially. This supersedes the
previous Warlord/Merc infighting policy and custom-faction proposal. It does
not merge patrols or change recruitment, equipment, coordinates, population
caps, movement, combat attributes, or spawn/lifecycle settings.

Stock retaliation and friendly assistance remain enabled and can draw
Civilians into nearby battles; faction friendship is not a guarantee they
never fight. Later staging checks must confirm separate Merc patrols remain
friendly, West patrols cooperate, East and West engage, players retain their
expected relationships, and Civilian retaliation/assistance behaves acceptably.
Warlord encounters require later patrol implementation.

Validation records:

- [Baseline server-root validation](validation/faction-alliances-baseline.txt).
- [Final server-root validation](validation/faction-alliances-final.txt).
- [Final mission-root validation](validation/faction-alliances-mission-final.txt).

Both final validators pass; server-root output matches the baseline. Static
comparison confirms only the 122 intended `Faction` values changed and all
other patrol fields and loadouts are preserved. These are configuration checks,
not in-game verification. No tests, custom scripts/mods, server launch, or
deployment are part of this update. Deploy the
updated mission `AIPatrolSettings.json` with staging stopped, then restart using
the existing launcher; retain existing profile data and loadouts.

## Western Merc object patrol phase (2026-10-20)

### Current status and population limits

The staging configuration now contains **143 patrol definitions**: the original
40 single-AI Civilian/Tribesman entries, **102 two-AI Merc entries**, and the
single-AI `assasin`. Its **45 loadout files** comprise the 42 original equipment
definitions, two Merc selectors, and `Nasdara_Assasin.json`. The object-patrol phase preserved the original 40 patrol entries,
all 42 equipment files, profile `AISettings.json`, launchers, and production
server. The later alliance update changes only the patrol factions described
above.
Warlord patrols and the six-step geometry scanner remain future work.

Load balancing applies across player counts 0-255:

| Category | Maximum patrols | AI per patrol | Maximum living AI |
| --- | ---: | ---: | ---: |
| Civilian | 20 | 1 | 20 |
| Tribesman | 20 | 1 | 20 |
| Western_Merc | 10 | 2 | 20 |
| Assasin | 1 | 1 | 1 |

`Global.MaxPatrols` is **61**. It counts patrols, not individual AI; the category
caps above enforce at most **51 active patrols / 61 living AI**. Unused category
capacity is not borrowed by another role. Player proximity, object eligibility,
and native spawn rules can keep the actual population below those ceilings.

### Object manifest and provenance

The exact selected class list, source order, placement counts, military-loot
usage, and CE revision are recorded in
[validation/merc-object-classes.csv](validation/merc-object-classes.csv).
All **51 classes** are included, including fuel tanks; the pinned official
placement file lists **460 objects** across those classes. This is a count of
listed source placements, not a measured count of runtime spawners or safe
spawn areas. Class-based matching can apply wherever those classes occur; it
does not enforce military-base boundaries.

Sources use CE commit `f220b2918b1d79a5055b1ee9be70c0ec1516e876`:

- [mapgrouppos.xml](https://github.com/BohemiaInteractive/DayZ-Central-Economy/blob/f220b2918b1d79a5055b1ee9be70c0ec1516e876/dayzOffline.nasdara/mapgrouppos.xml),
  SHA-256 `4c1c1ba3c58371866805ac9b9cd7f00363529d022f436ad859a282e3d552f05e`.
- [mapgroupproto.xml](https://github.com/BohemiaInteractive/DayZ-Central-Economy/blob/f220b2918b1d79a5055b1ee9be70c0ec1516e876/dayzOffline.nasdara/mapgroupproto.xml),
  SHA-256 `b687bead88ff2e33742c319f2b2fa9143dce08593489cfc06bb8a8a50c1f8967`.

All 51 class definitions and their inheritance were checked against installed
DayZ build **25956772** without launching DayZ. Class availability does not
establish terrain clearance, object placement safety, or deployed-version parity.

### Profile selection and equipment

Each class has consecutive `BoltAction` and `Regular` definitions, named
`Nasdara_Western_Merc_O01_BoltAction` / `..._Regular` through `O51`, in manifest
order. Each uses its `ObjectClassName` with empty `Waypoints`; stock Expansion
creates positions around the matching object using a 40-80 m native spread.
No world coordinates were added.

The two selector files use stock mutually exclusive, same-name `Sets` and
`Include` references, choosing uniformly among their candidates independently
for each AI on each spawn:

| Selector filename without `.json` | Existing equipment definitions |
| --- | --- |
| `Nasdara_Western_Merc_BoltAction_Random` | `Nasdara_Western_Merc_10` (SV98) and `Nasdara_Western_Merc_14` (Scout) |
| `Nasdara_Western_Merc_Regular_Random` | The other 14 Merc definitions, including `Nasdara_Western_Merc_11_Medic` |

Expansion takes the first successful object definition. The BoltAction entry
comes first with `Chance: 0.0625`; Regular follows with
`Chance: 0.4666666666666667`. Thus an object has nominal 50% eligibility:
`0.0625 + (1 - 0.0625) * 0.4666666666666667 = 0.5`.
Conditional on selection, the profile weights are 12.5% BoltAction and 87.5%
Regular. Together with uniform selectors this gives each original kit equal
nominal 1/16 weighting among selected AI, not equal observed active counts.

Object eligibility and profile choice occur when the object spawner initializes.
Respawns retain that profile while rerolling each member's equipment within its
selector. Both members share the profile, but can receive different kits.
The original equipment files, including the Scout's guaranteed 4x ACOG and
medic supplies, are preserved.

### Merc patrol settings and native limits

| Setting | Value |
| --- | --- |
| `Faction` / `LoadBalancingCategory` | `East` / `Western_Merc` |
| `NumberOfAI` / `NumberOfAIMax` | 2 / 0 |
| `Behaviour` / `Speed` / `UnderThreatSpeed` | `ROAMING_LOCAL` / `JOG` / `SPRINT` |
| `AccuracyMin` / `AccuracyMax` | 0.45 / 0.95 |
| `ThreatDistanceLimit` / `NoiseInvestigationDistanceLimit` | 200 / 100 m |
| `MaxFlankingDistance` / `EnableFlankingOutsideCombat` | 200 m / 1 |
| `SniperProneDistanceThreshold` | BoltAction 190 m; Regular 0 |
| `MinDistRadius` / `MaxDistRadius` / `DespawnRadius` | 250 / 1000 / 1200 m |
| `MinSpreadRadius` / `MaxSpreadRadius` | 40 / 80 m |
| `DespawnTime` / `RespawnTime` | 300 / 1800 seconds |
| `CanBeLooted` / `UnlimitedReload` | 1 / 1 |
| `Persist` / `CanBeTriggeredByAI` / `CanSpawnInContaminatedArea` | 0 / 0 / 0 |

Scavenging is explicitly
`WEAPONS_FIREARMS | WEAPONS_MELEE | BANDAGES | CLOTHING | UPGRADE`.
Other fields retain the existing patrol template's defaults. Merc pair members
and separate Merc patrols cooperate through stock `East`. They fight West
players and future Mercenaries Warlords, while initially accepting Civilian
and Tribesman patrols. Stock retaliation and friendly assistance still apply.

Automatic sniper-prone shooting requires qualifying optics, facing the target,
and no recent hit. The SV98's MK4 qualifies; the Scout's required 4x ACOG does
not (native optic ranges 1100 m versus 600 m, with an 800 m qualification
threshold). Regular profiles disable this automatic sniper-prone behavior,
not every possible use of prone cover. Scavenging never reassigns the profile.

`Persist: 0` is required for stock object patrols. Spawn/respawn/despawn timers
are native lifecycle settings, not guarantees of continuous presence or exact
wall-clock transitions. Local roaming can leave compounds and has no enforced
military-base leash. Empty waypoints, native height resolution, and spread do
not establish flat, outdoor, unobstructed ground or safe elevation. Multiple
matching structures in one base can generate multiple eligible spawners.

### Configuration checks and later staging acceptance

Repo-local `dzcli` v0.0.5 baseline and final checks cover both the server root
and nested mission root. Supported mutations use reviewed `--dry-run` previews;
unsupported patrol-root/category changes use direct edits. Validation records:

- [Baseline](validation/merc-object-baseline.txt).
- [Final validator output](validation/merc-object-final.txt).
- [Static audit](validation/merc-object-audit.json).

That historical object-patrol audit covers JSON parsing, 142 patrols,
44 loadout files, exact 51-class coverage/order and placement-count sum, the two selector reference pools,
profile order/probability calculations, caps, all Merc overrides, and preservation
of original equipment, patrol entries, AI settings, launchers, and production.
These are configuration checks; they do not establish in-game behavior.
No tests, custom scripts/mods, server launch, or deployment are part of this phase.

For deployment, stop staging, copy the updated mission patrol file and all
loadouts (including both selectors) into the existing installed overlays, retain
unrelated mission/profile data, then restart using the existing launcher.
Do not remove native stock mission data or persistence as part of this update.

Later staging acceptance remains pending:

- [ ] Confirm outdoor ground support, clearance, elevation, walking exits, and
      40-80 m spread around selected structures; inspect any rejected spawns.
- [ ] Confirm pairs, distinct per-member equipment rolls, selector references,
      medic inventory, and all 16 kit possibilities across sufficient spawns.
- [ ] Confirm category limits, the current 61-AI ceiling, object eligibility
      across restarts, and activation at 250-1000 m as players move among dense bases.
- [ ] Confirm 1200 m despawn range and native 300/1800-second lifecycle behavior.
- [ ] Compare 200 m threat range, 100 m investigation, 200 m flanking, JOG/SPRINT,
      and outside-combat flanking under controlled encounters.
- [ ] Confirm eligible SV98 prone shooting beyond 190 m, Scout/Regular exceptions,
      prone cover, and unchanged behavior profile after scavenging a new weapon.
- [ ] Confirm unlimited reload, lootable corpses, scavenging/clothing changes,
      cooperation within/between Merc patrols, hostility toward players and
      future Warlords, and initial friendship with both Civilian-faction roles
      with stock retaliation/assistance.
- [ ] Check local roaming departures and host performance; no base boundary or
      safe-placement scanner is implemented by these object definitions.

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

The initial patrol phase used the user's approved configuration-only alternative:
copy the official player spawn centers and let stock Expansion resolve height
and apply small spawn spreads. No new coordinates, geometry scanner, custom
mod, launcher changes, dependency installation, or game/server launch were part
of that original phase. Staging launcher activation was added after relocation;
the centers have not passed the scanner acceptance rules below.

## Current evidence and constraints

- The staging launcher loads CF, VPPAdminTools, Dabs Framework, Expansion Core,
  and Expansion AI. The operator reports a successful prior deployment/startup
  on a separate VM and the correct DZSA mod list. Object-patrol runtime checks
  remain pending. See [README.md](README.md#expansion-ai-configuration).
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
  Civilian 1-2 m and Tribesman 4-5 m native spread, now with one AI per patrol.
- Preserve source provenance, map/build information, configuration, results,
  and checkpoints so a scan can be audited and resumed.
- Keep target counts and area quotas configurable. A request for 300 accepted
  locations is an example, not an agreed requirement.
- Report an explicit shortfall when an area's search budget is exhausted.
  Individual rejected points do not become operator review tasks.

## Implemented configuration phase

This historical section records the original ten-loadout/40-patrol phase.
The later loadout-only expansion preserved those patrols; the current object
phase adds Merc patrols and supersedes the original global/category cap summary.

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
inherit the root/global defaults where appropriate. The agreed combat tiers
below are planned overrides for Civilian/Tribesman/Warlord. Merc overrides
are now explicit in the object patrols. The alliance update later changes only
Tribesman factions in these original entries.

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

Each of the original six Tribesman loadouts also has one guaranteed loose-ammo stack; there are no ammo boxes
or extra ammo types:

| Firearm | Ammo class | Quantity Min/Max | Initial rounds at native stack capacity |
| --- | --- | --- | --- |
| Izh18/Izh43 | `Ammo_12gaPellets` | 0.20 / 0.20 | 2 (10-round shell stack) |
| Mosin | `Ammo_762x54` | 0.25 / 0.25 | 5 (20-round stack) |
| SKS/SKS_Desert | `Ammo_762x39` | 0.40 / 0.40 | 8 (20-round stack) |

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

This table records the original singleton phase. Current patrols again use
one AI each, with the global cap now 61, scavenging disabled and Chance 0.75.
Both Civilian/Tribesman category caps remain 20 patrols each.

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

The original phase enabled Civilian scavenging through
`WEAPONS_FIREARMS | WEAPONS_MELEE | BANDAGES | CLOTHING | UPGRADE` and Tribesman
scavenging through `WEAPONS_MELEE | BANDAGES | CLOTHING | UPGRADE`.
The current no-scavenging update supersedes both allowlists with
`LootingBehaviour: NONE`; both roles now retain their starting equipment
without looting replacements. Corpses remain lootable through `CanBeLooted: 1`.

`PlayerFactions: ["West"]` supplies mutual friendship with stock Civilian
patrols. Existing persisted player groups can already have a faction; the
connect-time default does not guarantee reassignment of those groups.
Both `CanRecruitFriendly` and `CanRecruitGuards` are 0 so players cannot recruit
these individuals into larger patrols. Stock retaliation, aggression expiry,
and assistance to friendly patrols are preserved. This phase adds neither the
custom targeting rules nor Warlord/Western_Merc patrols.

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
The original phase's launchers activated only CF and VPPAdminTools. The staging
launcher later added the required AI dependencies; the operator reports a
successful deployment/startup of that earlier overlay.

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

## Loadout expansion phase (2026-10-20)

### Scope, naming, and provenance

This historical phase added **32 loadout files**: `Nasdara_Warlord_01`-`13`,
`Nasdara_Western_Merc_01`-`16` (11 is named `Nasdara_Western_Merc_11_Medic`),
and `Nasdara_Tribesman_07`-`09`, each with a `.json` extension in `profiles/ExpansionMod/Loadouts/`. Together with the
original ten definitions, there are **42 loadouts**. The original four Civilian
and six Tribesman files are preserved byte for byte. These new definitions are
**not assigned to patrols in that phase**. The current Merc object phase now
references all 16 Merc kits through selectors. The existing 40 patrols, their coordinates,
settings, loadout cycling, faction assignments, and the planned combat tiers
are unchanged. That loadout-only phase added no launchers, mods, or custom
faction implementation. Staging launcher dependencies were added later.

The explicit inventory class allowlist remains the pinned official Nasdara
`types.xml` above. Weapon compatibility, native capacities, slots, quantities,
and food states were inspected in memory from the installed DayZ Steam build
**25956772**, without starting DayZ. Native implicit `AugShort` scope and SCAR front
sight behavior are the approved exception to the allowlist: these are engine
weapon parts, not manually added unlisted inventory items. Named Expansion
sets are probability groups rather than item class references.

All new starting items are pristine. Ordinary finish alternatives are uniform;
the Warlord KA-M gold branch is the explicit 1% exception. Full detachable
magazines use normalized `Quantity.Min/Max: 1/1`. Internal-magazine firearms
receive loose cartridges for native AI loading; immediate loaded/chambered
readiness is not established by configuration validation.

### Tribesman .380 exception

| New loadout | Existing outfit/supplies copied from | Firearm | Guaranteed full attached magazine | Other weapon parts |
| --- | --- | --- | --- | --- |
| `Nasdara_Tribesman_07` | `Nasdara_Tribesman_01` | `CZ61` | `Mag_CZ61_20Rnd` (20) | Native fixed parts |
| `Nasdara_Tribesman_08` | `Nasdara_Tribesman_02` | `PM73Rak` | `Mag_PM73_15Rnd` (15) | Native fixed parts |
| `Nasdara_Tribesman_09` | `Nasdara_Tribesman_03` | `PP19` | `Mag_PP19_64Rnd` (64) | `PP19_Bttstck` |

These three additions provide .380 ammunition representation. They receive no
spare magazines, loose ammunition, optics, suppressors, or food. Outfit pools
and existing optional helper probabilities are copied from their stated base
loadouts. The six original firearms remain restricted to their approved
shotgun/Mosin/SKS loadouts. Existing patrol assignments still cycle 01-06.
The original shotgun quantity 0.20 means **two shells**, because native
`Ammo_12gaPellets` stacks contain ten; the original JSON quantity is unchanged.

### Warlord weapons, ammunition, and attachments

| Suffix | Firearm family | Ammunition | Starting ammunition | Optional weapon equipment |
| --- | --- | --- | --- | --- |
| 01 | KA-74 (`AK74` family) | 5.45x39 | Full attached 30-round magazine; 1 or 2 full spares | Bayonet 50%; mounted light 25% with compatible rail handguard |
| 02 | KA-74U (`AKS74U` family) | 5.45x39 | Full attached 30-round magazine; 1 or 2 full spares | Required native buttstock |
| 03 | KA-M (`AKM` family) | 7.62x39 | Full attached 30-round magazine; 1 or 2 full spares | Bayonet 50%; ordinary finishes may have mounted light 25% |
| 04 | VSD (`SVD`) | 7.62x54 | Full attached 10-round magazine; 1 or 2 full spares | Iron sights; no optic or suppressor |
| 05 | Mosin (`Mosin9130` family) | 7.62x54 | 20 loose rounds | PU scope 30%; bayonet 30%, compensator 30%, neither 40% |
| 06 | SKS family | 7.62x39 | 20 loose rounds | PU scope 30%; bayonet 50% |
| 07 | Lee Enfield | 7.62x54R | Full attached 10-round magazine; 1 or 2 full spares | 1907 bayonet 50% |
| 08 | Pump shotgun (`Mp133Shotgun`) | 12 gauge | 20 loose buckshot shells | Native fixed parts |
| 09 | Double barrel (`Izh43Shotgun`) | 12 gauge | 20 loose buckshot shells | Native fixed parts |
| 10 | Single barrel (`Izh18Shotgun`) | 12 gauge | 20 loose buckshot shells | Native fixed parts |
| 11 | `CZ61` | .380 | Full attached 20-round magazine; 1 or 2 full spares | Native fixed parts |
| 12 | `PM73Rak` | .380 | Full attached 25-round magazine; 1 or 2 full spares | Native fixed parts |
| 13 | `PP19` | .380 | Full attached 64-round magazine; 0 or 1 full spare | `PP19_Bttstck`; guaranteed `CanvasBag_Olive` |

For ordinary detachable magazines, one and two spares each have a 50% chance.
Bizon zero/one spare is also 50/50. No 75-round KA-M magazines or shotgun drums
are included. Loose rifle ammunition is one 20-round stack; loose shotgun
ammunition is two full ten-shell `Ammo_12gaPellets` stacks. No ammunition boxes
or secondary firearms are added. Warlords have no suppressors. PU scopes on
Mosin/SKS are their only possible optics. KA-74/KA-M handguards and buttstocks
are complete and compatible; gold KA-M uses matching gold/dark intricate
furniture and its full gold 30-round magazine, with no mounted light.

KA-M chooses ordinary finishes with a combined 99% chance and gold with 1%.
Its JSON gold candidate eligibility is 0.02 against an always-eligible ordinary
candidate with the same set name. When both are eligible, stock Expansion picks
one uniformly, so the effective gold probability is 0.02 x 0.5 = 0.01. The raw
2% eligibility is not the final spawn probability.
Ordinary finish candidates are uniform within their branch. Other supported
weapon finish alternatives are uniform. Magazine/furniture choices match
weapon compatibility; the gold magazine and furniture remain in its branch.

### Warlord clothing and supplies

One of the five traditional top/pants pairs from Tribesman_01 through _05 is
chosen uniformly, together as an outfit. Existing regional headwear and sandal
pools are retained. These Warlord outfit choices do not include the OBR uniform.

| Vest outcome | Probability | Selection |
| --- | --- | --- |
| No vest | 30% | No vest is created |
| Chest rig | 40% | Uniform `ChestRigVest_Khaki`, `_Olive`, `_Tan` |
| Press ballistic vest | 20% | Uniform blue/light-blue variants |
| Plate carrier with pouches | 10% | Compatible carrier and pouch pair |

Only the Bizon Warlord always receives `CanvasBag_Olive`; other Warlords have no
backpack. Each loadout independently rolls one `BandageDressing` at 50%, one
`PersonalRadio` at 25%, one `Flashlight` at 25%, and one water container at 50%.
A successful water roll selects `Canteen` or `WaterBottle` uniformly, filled to
35-65% capacity using native liquid defaults. Radio and flashlight rolls include
one full `Battery9V` attached to that device. They do not spawn loose batteries
or a second water container.

### Western Mercenary weapons and attachments

| Suffix | Firearm | Caliber | Removable optic policy | Removable suppressor policy |
| --- | --- | --- | --- | --- |
| 01 | M4 (`M4A1` family) | 5.56x45 | 90% western AR pool | 60% `M4_Suppressor` |
| 02 | `M16A2` | 5.56x45 | Native sights only | 60% `M4_Suppressor` |
| 03 | `FAMAS` | 5.56x45 | Native sights only | None supported |
| 04 | `Aug` | 5.56x45 | 90% western AR pool | 60% `M4_Suppressor` |
| 05 | `AugShort` | 5.56x45 | Integral native scope | None supported |
| 06 | SCR16 (`SCARL` family) | 5.56x45 | 90% western AR pool | 60% `M4_Suppressor` |
| 07 | SCR17 (`SCARH` family) | .308 | 90% .308 pool | None supported |
| 08 | `FAL` | .308 | 90% .308 pool | None supported |
| 09 | `M14` | .308 | 90% .308 pool | None supported |
| 10 | `SV98` | 7.62x54 | 100% `MK4Optic_Green` | None supported |
| 11_Medic | `MP5K` (medic) | 9x19 | 90% Reflex | 60% `PistolSuppressor` |
| 12 | `ASVAL` | 9x39 | 90% Reflex/4x ACOG, uniform | Integral suppression always present |
| 13 | `Vikhr` | 9x39 | 90% Kashtan/PSO-1, uniform | None supported |
| 14 | `Scout` family | 5.56x45 | **100% 4x `ACOGOptic`** | 60% `M4_Suppressor` |
| 15 | `R12` | 12 gauge | 90% Reflex | None supported |
| 16 | `Saiga` | 12 gauge | 90% Kobra | None supported |

The western AR optic pool is `M68Optic`, `ReflexOptic`, `M4_T3NRDSOptic`, and
4x `ACOGOptic`, uniformly selected
when the 90% optic roll succeeds. The .308 pool is 4x/6x ACOG, uniformly selected.
SV98 always receives its compatible `MK4Optic_Green` and full battery. SV98 has no compatible
stock removable suppressor; no improvised suppressor is substituted. The
unsupported suppressor entries above are explicit exclusions. The VSS proposal
was removed; there is only one SVAL/ASVAL loadout.

Every detachable-magazine weapon has one full attached magazine plus exactly
2, 3, or 4 full spare magazines with equal probability. SVAL uses the compatible
30-round Vikhr magazine, Vikhr uses 30-round magazines, and Saiga uses eight-round
magazines. No high-capacity STANAG alternatives or drums are used. R12 instead
has 40 loose buckshot shells in four full ten-shell stacks. Each loadout uses
one ammunition caliber. No pistols or additional firearms are included.

| Merc suffixes | Full magazine class / capacity |
| --- | --- |
| 01, 02, 06 | `Mag_STANAG_30Rnd` / 30 |
| 03 | `Mag_FAMAS_25Rnd` / 25 |
| 04, 05 | `Mag_Aug_30Rnd` / 30 |
| 07 | `Mag_SCARH_20Rnd` or `_Black`, matching finish / 20 |
| 08 | `Mag_FAL_20Rnd` / 20 |
| 09 | `Mag_M14_20Rnd` / 20 |
| 10 | `Mag_SV98_10Rnd` / 10 |
| 11_Medic | `Mag_MP5_30Rnd` / 30 |
| 12, 13 | `Mag_Vikhr_30Rnd` / 30 |
| 14 | `Mag_Scout_5Rnd` / 5 |
| 15 | No detachable magazine; four full buckshot stacks / 40 shells |
| 16 | `Mag_Saiga_8Rnd` / 8 |

M4 finish alternatives are `M4A1`, `M4A1_Black`, and `M4A1_Green`; SCR16 and
SCR17 choose their standard/black variants, with matching stocks and SCR17
magazines. Scout chooses `Scout`, `Scout_Chernarus`, or `Scout_Livonia`.
Each of these finish pools is uniform; other Merc firearms use their fixed
approved class. M4 has `M4_RISHndgrd` and `M4_MPBttstck`; other removable
furniture follows each weapon's native compatible slots.

All required handguards and stocks are present. Compatible mounted lights roll
at 75% for M4, MP5, AUG, SCR16, SCR17, ASVAL, and R12. M4/M16 bayonets roll at
75% only when no suppressor is selected; the MP5 compensator similarly rolls
at 75% only without its suppressor. M4 BUIS is present only in the 10% branch
without a removable optic. Scoped SV98 and Scout guarantees supersede the
ordinary 90% optic rule.

### Western Mercenary clothing, utilities, and medical equipment

Every Merc wears matching `USMCJacket_Desert` and `USMCPants_Desert`, a guaranteed
coyote `PlateCarrierVest` with `PlateCarrierPouches`, `Mich2001Helmet`, `MilitaryBoots_Beige`, and
`TacticalGloves_Beige`. `CoyoteBag_Desert` is the guaranteed ordinary backpack. NVGs have a 1% chance and
include a full battery. The medic retains the same uniform, helmet, boots, and
plate carrier but uses `SurgicalGloves_White`, `SurgicalMask`,
`CanvasBag_Medical`, and `HipPack_Medical`.

All Mercs receive a knife and one water container. Water selects `Canteen` or
`WaterBottle` uniformly at 75-100% capacity. Optional utilities are a radio at
75%, flashlight at 50%, and rangefinder at 20%; each powered item includes a
full attached battery. Radio uses the backpack's compatible external radio slot,
rangefinder its compatible chemlight slot, and `CombatKnife` the military
boots' knife slot. Utility rolls are independent.

| Medical item | Ordinary Merc | MP5 medic |
| --- | --- | --- |
| `BandageDressing` | 1 full roll, guaranteed | 3 full rolls, guaranteed |
| `StartKitIV` | 1 at 25% | 2 guaranteed |
| `BloodBagEmpty` | 1 at 25% | 2 guaranteed |
| `SalineBag` | 1 at 25% | 2 guaranteed |
| `Epinephrine` | 1 at 25% | 2 guaranteed |
| `Morphine` | 1 at 25% | 2 guaranteed |
| `TetracyclineAntibiotics` | 1 full pack at 25% | 1 full 12-tablet pack, guaranteed |
| Each of the 12 additional items below | 1 at 25%, independently | 1 at 25%, independently |

The 12 additional medical items are `BloodTestKit`, `CharcoalTablets`,
`PainkillerTablets`, `VitaminBottle`, `PurificationTablets`, `ChelatingTablets`,
`DisinfectantAlcohol`, `DisinfectantSpray`, `IodineTincture`, `Thermometer`,
`Splint`, and `AntiChemInjector`. Thus ordinary Mercs have 18 independent
optional medical rolls, while the medic has 12. No rags, heatpacks, first-aid
containers, prefilled blood bags, or prepared `SalineBagIV` items are added.

A full bandage roll has four uses. Native full packs have 12 tetra/charcoal/
painkiller tablets, 50 vitamins, 10 purification tablets, or 20 chelation tablets.
Other medical items preserve native initial quantities. Empty blood bags stay
empty. A separate `StartKitIV` and `SalineBag` must be combined to make a usable
saline IV, consuming both; using both medic IV kits for saline leaves none for
collected blood. Chelation tablets are allowlisted even though their pinned CE
nominal is zero. Carrying medical items does not establish autonomous AI use:
stock self-bandaging is supported, while advanced treatment and medic healing
behavior have not been implemented or demonstrated. These supplies are carried
inventory and potential loot.

#### Medic container bindings

Cargo is explicitly assigned to its container so optional items cannot fill a
required item's space in another container. The native cargo capacities below
total 90 cells; quantities and independent optional chances remain as specified
in the medical table. The worst case includes all optional medicines, four
spare MP5 magazines, flashlight, canteen, and a four-cell food item.

| Medic container | Native cargo | Worst-case used | Assigned contents |
| --- | --- | --- | --- |
| `USMCJacket_Desert` | 6x4 (24 cells) | 22 | 3 bandage rolls; 2 IV kits; 2 empty blood bags; 2 epinephrine; 2 morphine |
| `USMCPants_Desert` | 5x4 (20 cells) | 19 | 2 saline bags; optional vitamins, purification, chelation, alcohol, iodine, blood test kit |
| `PlateCarrierPouches` | 5x3 (15 cells) | 15 | 2/3/4 spare MP5 magazines; optional disinfectant spray |
| `CanvasBag_Medical` | 5x5 (25 cells) | 23 | Optional splint, thermometer, PO-X injector and flashlight; guaranteed water choice; single 5% food choice |
| `HipPack_Medical` | 3x2 (6 cells) | 6 | Guaranteed tetra pack; optional charcoal and painkillers |

The worst case is **85/90 cells** with a canteen, or 84/90 with a water bottle.
The radio attaches externally to the medical backpack's `WalkieTalkie` slot and
the rangefinder to its compatible `Chemlight` slot. Both include their batteries
and use no backpack cargo cells. The knife attaches to the military boots'
`Knife` slot; helmet NVGs attach to `NVG`. Weapon-mounted optics and light carry
their own attached batteries. These bindings express the intended packing;
engine spawning and physical inventory acceptance still require runtime checks.

### Food probability and native state

Every Warlord and Merc, including the medic, has **one 5% food roll**. A successful
roll selects exactly one item uniformly from its role's pool:

| Role | Food alternatives | Starting state |
| --- | --- | --- |
| Warlord | `HalvaCan`, `SarmaCan` | Sealed, native contents; 250 g after opening |
| Merc | `TacticalBaconCan`, `Crackers`, `ZagorkyChocolate` | Sealed can or full native snack; 400 g/50 g/100 g respectively |

An outer 5% Expansion set gates named, guaranteed alternatives so there is one
uniform choice, rather than several independent 5% rolls. Closed can quantity
0/0 preserves its native state; it does not mean an empty can. Snacks retain
native full quantities. No food is added to Civilian or Tribesman loadouts.
Figs were excluded because they are perishable fruit and loadout JSON cannot
guarantee their dried food stage.

### Static checks and deferred runtime acceptance

The phase uses repo-local dzcli v0.0.5. All **2,544 configuration mutations**
(1,076 Warlord, 1,458 Merc, and 10 Tribesman) used supported CLI commands, each
with a `--dry-run` preview, a text-mode application, and comparison of the written
JSON with its preview. JSON-mode mutations do not write in this version. The
three Tribesman additions first copy approved existing files because the CLI
has no clone command. No direct JSON settings/probability edits were needed. No
new tests were added and no server/game was launched.

Expanded baseline/final validation and the static audit are recorded separately
as `validation/loadout-expansion-baseline.txt`,
`validation/loadout-expansion-final.txt`, and
`validation/loadout-expansion-static-audit.json`. **Baseline and final profile/
mission validations pass**, as does the independent static audit. The baseline
report is an explicitly labeled summary of observed pre-edit command results,
not a reconstructed raw transcript. The final report contains command output.
The audit confirms 32 additions, 42 total loadouts, 180 distinct explicit
inventory classes across the new files, all in the pinned allowlist, analytic
probabilities and native compatibility, and preservation of existing JSON.
Original reports above remain historical evidence for the ten-loadout patrol
phase; their 45-class count describes that earlier subset.

Required static checks cover JSON parsing, 42 loadouts and 32 additions, explicit
class allowlist membership, weapon/magazine/attachment compatibility, native
quantities, full magazines, all probability branches, one food outcome, medical
counts, and unchanged original files/settings/patrols. Native configuration
inspection and packing analysis do not establish in-game inventory placement.
The medic's worst case was statically packed at **85/90 cells**, including all
optional outcomes and the largest selected food; this remains a feasibility
calculation rather than an engine placement guarantee.

Later runtime acceptance must check all optional branches, clothing/attachment
slots and cargo capacity, full magazine attachment and spare accessibility,
internal-magazine/chamber loading, RAK open-bolt readiness, reloads, supported
optics and suppressors, integral weapon parts, light/NVG batteries, medical
quantities and looting, and sealed food behavior. The original placement,
movement, detection, combat-tier and faction acceptance work remains deferred.

## Faction and loadout design

### Confirmed gameplay policy

The requested setting uses Tribesmen, Civilians, Warlords, and `Western_Merc`
patrols. `Western_Merc` retains NATO-style equipment. The current player
relationship update supersedes the earlier West alliance: Tribesmen and
Civilians use `Civilian`; future Warlords use stock `Mercenaries`; Mercs use
`East`; players remain `West`. Warlord patrols are still future work. No custom
faction is required for this agreed baseline policy. The additional `assasin`
uses `Raiders` and is hostile to all these roles and players, including Civilians.

| Acting patrol | Tribesmen | Civilians | Warlords | Western_Merc | Players |
| --- | --- | --- | --- | --- | --- |
| Tribesmen | Friendly* | Friendly* | Hostile | Friendly* | Friendly* |
| Civilians | Friendly* | Friendly* | Hostile | Friendly* | Friendly* |
| Warlords (planned) | Hostile | Hostile | Friendly* | Hostile | Hostile |
| Western_Merc | Friendly* | Friendly* | Hostile | Friendly* | Hostile |

This matrix describes expected baseline encounters: stock target evaluation
can respond to another faction's hostility even when `Civilian.IsFriendly`
returns true. Warlord relationships are planned until their patrols exist.

*Friendly describes baseline faction relationships. Stock retaliation and
assistance beyond the attacked patrol remain enabled. Helping friendly
patrols can pull Civilians into nearby battles; friendship is not immunity
from combat. Separate allied patrols remain separate groups and do not
recruit or merge. Assistance eligibility and aggression expiry retain stock
behavior (120 seconds, guard 150 seconds) and require runtime checks.

### Agreed combat tiers (Merc implemented; other roles pending)

The following selected values are ordered from lowest to highest accuracy.
Merc object patrols now explicitly implement tier 4; the original Civilian and
Tribesman accuracy inheritance is preserved, and Warlord patrols remain pending.

| Tier | Patrol role | `AccuracyMin` | `AccuracyMax` | `EnableFlankingOutsideCombat` |
| --- | --- | --- | --- | --- |
| 1 | Civilian | 0.25 | 0.75 | 0 (disabled) |
| 2 | Tribesman | 0.30 | 0.85 | 0 (disabled) |
| 3 | Warlord | 0.35 | 0.95 | 0 (disabled) |
| 4 | Western Mercenaries (`Western_Merc`) | 0.45 | 0.95 | 1 (enabled) |

Warlords use the inspected stock accuracy range. Western Mercenaries have a
higher minimum while retaining the stock maximum. These are selected gameplay
settings, not measured hit percentages or verified combat outcomes.

Future implementation must set explicit `AccuracyMin` and `AccuracyMax`
overrides on the remaining roles, independently of their assigned stock
faction. Merc patrols already use 0.45/0.95 and
`EnableFlankingOutsideCombat: 1`. It remains disabled globally and on every
existing Civilian/Tribesman patrol; keep it disabled for future Warlords.
All roles may still flank in combat.

Later validation must check the assigned settings for every patrol and compare
combat behavior with matching weapons, distances, and target movement across
the four roles. Western Mercenaries retain outside-combat flanking; the
additional `assasin` also enables it. Civilians, Tribesmen, and future Warlords
keep it disabled.
The Merc configuration is implemented. The separate `assasin` role uses
0.85/0.95 accuracy and does not alter the four agreed tiers above. Remaining
role overrides and all comparative runtime checks are deferred.

### What existing Expansion factions provide

Research used public `release` commit
`0d26645ca31bc574bc3ebee1d4bc9192727f3c43`. The 13 inspected faction, patrol,
targeting, settings, and player integration files matched their counterparts
at `experimental` commit `14fd692bd7eb46df05853ea31ee3d481239a826c`.
This is source inspection; no game session or faction encounter was run.

| Role | Starting point | Required work or limitation |
| --- | --- | --- |
| Tribesmen | `Civilian`, with armed local loadouts | Implemented on all 20 patrols; initially friendly to West players, Civilians and East Mercs; future Mercenaries Warlords are hostile. |
| Civilians | `Civilian`, with civilian loadouts | All 20 patrols retain tool-only starting inventory; scavenging is disabled with LootingBehaviour NONE. |
| Warlords | `Mercenaries`, with existing Warlord loadouts | Required assignment for future patrols; cooperate across Warlord patrols, attack players, East Mercs and Civilian-faction roles. |
| Western_Merc | `East`, with Western military loadouts | Implemented on all 102 object definitions; separate Merc patrols cooperate. |

- `Civilian.IsFriendly` returns true for every faction. `East` and `West`
  accept their own faction and Civilian-derived factions, and oppose each
  other. Both Civilian equipment roles therefore initially accept West players
  and East Mercs. Hostile targeting and retaliation can still cause combat.
- Stock `Mercenaries` accepts its own faction and is hostile to West, East,
  and Civilian factions. This supplies Warlord cooperation and hostility to
  both players and Mercs without custom faction scripts.
- `Raiders` previously allowed Merc infighting and broad hostility. Mercs now
  use `East`; the independent `assasin` retains `Raiders`. The original custom
  Warlord proposal and later West Warlord alliance are superseded.

Sources: [Civilian](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/0d26645ca31bc574bc3ebee1d4bc9192727f3c43/DayZExpansion/AI/Scripts/3_Game/DayZExpansion_AI/Factions/eAIFactionCivilian.c),
[East](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/0d26645ca31bc574bc3ebee1d4bc9192727f3c43/DayZExpansion/AI/Scripts/3_Game/DayZExpansion_AI/Factions/eAIFactionEast.c),
[Raiders](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/0d26645ca31bc574bc3ebee1d4bc9192727f3c43/DayZExpansion/AI/Scripts/3_Game/DayZExpansion_AI/Factions/eAIFactionRaiders.c),
[Mercenaries](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/0d26645ca31bc574bc3ebee1d4bc9192727f3c43/DayZExpansion/AI/Scripts/3_Game/DayZExpansion_AI/Factions/eAIFactionMercenaries.c),
[custom faction registration guide](https://github.com/salutesh/DayZ-Expansion-Scripts/wiki/%5BModding%5D-Adding-a-new-Faction).

### Targeting and retaliation limits

Mercs use `East`, which initially accepts both Civilian-faction equipment
roles. Players remain `West`; assigning players to `Raiders` would make
Civilian-faction AI treat them as enemies through stock target evaluation.
Future Warlords use `Mercenaries` and can attack both neutral populations.
Stock retaliation and friendly assistance remain enabled; this mapping does
not guarantee Civilians or Tribesmen never join combat. Exact permanent
noncombatant behavior is not part of this update.

The hit handler can alert friendly AI outside the victim's group.
`AddFriendlyAI` requires mutual faction friendship for those other groups and
can propagate hostility to AI already aware of the aggressor. This supports
cooperation across Merc groups and across initially friendly neutral groups,
subject to runtime limits; it does not guarantee every friendly patrol across
the map joins a fight. Later encounters must check both Civilian-faction roles
near fighting players/Mercs, player attacks on initially friendly groups, and
future Warlord hostility.

Sources: [enemy selection](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/0d26645ca31bc574bc3ebee1d4bc9192727f3c43/DayZExpansion/AI/Scripts/4_World/DayZExpansion_AI/Entities/AI/eAIBase.c#L969),
[hit response](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/0d26645ca31bc574bc3ebee1d4bc9192727f3c43/DayZExpansion/AI/Scripts/4_World/DayZExpansion_AI/Entities/DayZPlayerImplement.c#L472),
[friendly assistance](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/0d26645ca31bc574bc3ebee1d4bc9192727f3c43/DayZExpansion/AI/Scripts/4_World/DayZExpansion_AI/Classes/Targets/eAITargetInformation.c#L388).

### Equipment and player relationships

`Faction`, patrol `Name`, and `Loadout` are separate fields. A populated
`Loadout` overrides the faction's default equipment. Tribesmen and Civilians
share `Civilian` while using different equipment files; future Warlords use
stock `Mercenaries` with their existing regional equipment. A patrol
named `Western_Merc_01` uses `Faction: East` and a Western loadout; a
literal `Faction: Western_Merc` requires registering that faction first.

The current Civilian role starts without firearms and uses
`LootingBehaviour: NONE`, preventing firearm pickup and other scavenging.
Tribesmen also use `NONE` and retain their starting firearms. Both roles have
explicit patrol overrides; default looting permits weapons and roaming can
default to `ALL` when no override is supplied.
Unarmed AI may still use melee when hostile; fleeing-only behavior is not
established by omitting a gun. `Passive` and `Observers` are unsuitable for the
requested normal retaliation role.

The approved player assignment remains `PlayerFactions: ["West"]`, giving
players initial friendship with Civilians/Tribesmen and hostility with East
Mercs and future Mercenaries Warlords.
The `West` name does not require NATO equipment or a NATO patrol role.
Existing persistent player groups may already have a faction and must be
considered separately.

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
The original phase implemented ten approved loadouts; the loadout expansion
above increases the prepared set to 42.

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
unlisted explicit inventory classes are excluded even when the engine supplies
them. The later approved exceptions are native implicit AUG/SCAR parts, as
documented in the loadout expansion section. Item
presence does not demonstrate Expansion AI weapon handling or a working loadout.

Sources: [Badlands release notes](https://dayz.com/article/general/dayz-badlands-out-now/dayz-steam-patch-notes),
[Road to Badlands release](https://dayz.com/article/game-update/dayz-update-129-road-to-badlands),
[Nasdara item definitions](https://github.com/BohemiaInteractive/DayZ-Central-Economy/blob/f220b2918b1d79a5055b1ee9be70c0ec1516e876/dayzOffline.nasdara/db/types.xml),
[earlier Chernarus definitions](https://github.com/BohemiaInteractive/DayZ-Central-Economy/blob/b86c1b71cbad4b62c4ca004f6ec7c8d4bbaccead/dayzOffline.chernarusplus/db/types.xml),
[1.30 scripts](https://github.com/BohemiaInteractive/DayZ-Script-Diff/tree/e3dda95b2b9730bdc0dba931abb35304b484bbfc).

### Implementation work identified by research

- [x] Select accuracy ranges and flanking outside combat policy for all four
      roles; see the agreed combat tiers above.
- [x] Apply explicit Merc accuracy and outside-combat flanking overrides to
      all 102 object definitions; Civilian/Tribesman combat settings are preserved.
- [ ] Future phase: apply the remaining per-role accuracy overrides and compare
      combat behavior under matching weapons, distances, and target movement.
- [x] Adopt Civilian for Tribesmen/Civilians, West players, East Mercs, and
      stock Mercenaries for future Warlords; supersede the earlier West
      Tribesman/Warlord alliance and custom faction proposals.
- [x] Define 13 Warlord and 16 Western_Merc loadouts, plus the three approved
      Tribesman .380 additions; all 16 Merc kits now have selector assignments.
- [x] Implement 102 Merc object definitions across all 51 selected classes,
      using stock East, paired AI, two behavior profiles, and category caps.
- [ ] Future phase: implement Warlord patrol assignments using stock
      Mercenaries under the approved player relationship policy; loadouts
      are already defined.
- [x] Configure West players, stock aggression expiry, and disabled recruitment
      for Civilian/Tribesman populations; runtime behavior remains unchecked.
- [x] Define ten starting loadouts and explicit per-role looting policies.
      Current Civilian/Tribesman scavenging is disabled with NONE; corpses stay lootable.
- [ ] Include encounter scenarios in the later automated runtime validation:
      each faction pair, player attacks, friendly assistance, separate allied
      groups, player/Merc hostility, future Warlord hostility toward both,
      and both Civilian-faction roles near dropped weapons.

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

1. Deploy the updated mission patrol file and `Nasdara_Assasin.json` to the
   existing staging host using the documented manual overlay copies; retain
   all existing loadouts and persistence.
2. Complete the assasin, Merc, and original Civilian/Tribesman runtime checklists.
3. When scanner work is authorized, establish the offline environment/query
   prerequisite, define its schemas and implement a small end-to-end batch.
4. Use that batch's evidence to refine Steps 1-6 before scaling to hundreds.

Current category caps are 20 Civilian singletons, 20 Tribesman singletons,
10 Merc pairs, and one assasin, with their approved native spreads. These permit
at most 51 patrols / 61 living AI. Warlord patrols are not yet implemented.
Future scanner decisions include area quotas, formation policy, minimum separation, numerical acceptance
thresholds, map-version matching, collision readiness detection, and packaging.

Follow the repository `dzcli` skill when inspecting or changing supported DayZ
configuration: bootstrap the local tool, inspect current values, preview
mutations with `--dry-run`, preserve baseline failures, and run matching
validators after changes. The six scanner steps remain unimplemented; the
current patrols use copied unverified centers, the official military-object
placement pool, and native object-relative placement rather than an accepted
scanner inventory.

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
| 2026-10-19 | Agreed accuracy tiers: Civilian 0.25-0.75, Tribesman 0.30-0.85, Warlord 0.35-0.95, Western_Merc 0.45-0.95; only Western_Merc enables flanking outside combat. | Record explicit per-role overrides for future implementation; configurations and runtime checks remain unchanged and deferred. |
| 2026-10-20 | Added 13 Warlord, 16 Western_Merc (including MP5 medic), and three Tribesman .380 loadout definitions. | Keep 42 prepared loadouts separate from the original 40 patrol assignments; all combat tiers, coordinates, behavior, factions, and deployment remain unchanged. |
| 2026-10-20 | Rebased PR #58 onto the staging baseline and moved all AI files/documentation into staging. Enabled Dabs Framework, Expansion Core, and Expansion AI in the staging launcher. | All 44 AI JSON files and six historical reports are preserved; production matches main. The baseline branch stays CF/VPP only. Launcher execution and runtime acceptance remain pending. |
| 2026-10-20 | Native build 25956772 inventory and compatibility inspected without launching DayZ. | Use ten-shell buckshot capacity, compatible full magazines and attachments, explicit medical quantities, single 5% food gates, and deferred engine acceptance; original six Tribesman JSON files remain unchanged. |
| 2026-10-20 | Operator reports merged work deployed on a separate staging VM, successful startup, and correct DZSA mod list. | Prior deployment is operator-reported; no object-patrol runtime verification is implied. |
| 2026-10-20 | Added all 51 military-related object classes (460 listed placements), 102 Merc definitions, and two uniform selectors. | Preserve the original 40 patrol entries and 42 kits; use 142 definitions / 44 loadout files and explicit Merc combat overrides. |
| 2026-10-20 | Global patrol cap raised to 60; category caps are Civilian 20, Tribesman 20, Merc 10. | Singletons plus Merc pairs yield a 60-living-AI ceiling and at most 50 active patrols; unused category capacity stays reserved. |
| 2026-10-20 | Alliance policy supersedes prior Warlord/Merc infighting and the custom Warlord faction proposal: Tribesmen and future Warlords use West; Mercs use East; Civilians remain Civilian; players remain West. | Change only Faction on 20 Tribesman and 102 Merc definitions. Warlord patrols remain deferred. Preserve stock retaliation/assistance and verify allied groups, West/East hostility, and Civilian responses later in staging. |
| 2026-10-20 | Add a single persistent hostile Raiders patrol named assasin, with fixed suppressed VSD/PSO6/desert-ghillie equipment and a uniform random start from 460 official military-object placements. | Add Assasin cap 1 and Global.MaxPatrols 61; retain existing caps and patrols. Use 143 definitions / 45 loadouts, ROAMING, 1000 m detection, 600 m sniper prone, 0.85/0.95 accuracy, bandage-only scavenging, and later runtime/persistence checks. |
| 2026-10-20 | Player relationship policy supersedes the earlier West Tribesman/Warlord alliance. | Change only Faction on 20 Tribesman patrols to Civilian; retain West players, Civilian civilians, East Mercs and Raiders assasin. Plan future Warlords as stock Mercenaries: hostile to players and Mercs, cooperative across Warlord groups. Preserve all other configuration and defer runtime checks. |
| 2026-10-20 | Replace assasin's 460 military-object start pool with one remote Land_Mausoleum_Small1_Dam1 anchor. | Select the most separated candidate among 324 placements of 138 uncommon CE classes: X 12170.946289 / Z 2173.485107, 1727.05 m from the nearest Merc object. Preserve every other setting; living persisted patrols move only on a future fresh spawn. |
| 2026-10-20 | Increase all 20 Civilian and 20 Tribesman patrols from singletons to pairs. | Set NumberOfAI 2 with NumberOfAIMax 0, retain 20/20 role patrol caps, and raise Global.MaxPatrols to 101. Current limits permit 51 patrols / 101 living AI; preserve the remote assassin anchor and every other setting. |
| 2026-10-20 | Disable town-loot scavenging for Civilians and Tribesmen. | Set LootingBehaviour NONE on all 40 definitions, superseding earlier allowlists. Preserve corpse lootability, Merc/assasin scavenging, pair sizes and the 101-AI ceiling. |
| 2026-10-20 | Restore Civilian and Tribesman patrols to one AI each while retaining disabled scavenging. | Set NumberOfAI 1 on all 40 definitions with NumberOfAIMax 0, retain 20/20 role caps, and restore Global.MaxPatrols 61. Category caps permit 51 patrols / 61 living AI. |
| 2026-10-20 | Reduce Civilian and Tribesman Chance to 0.75. Keep Tribesmen Civilian under the conditional Police request. | Stock Police is not a faction; Guards uses PoliceLoadout and different friendship/guard behavior. Change only 40 Chance values; preserve single-AI groups, disabled scavenging and the 61-AI ceiling. |
| 2026-10-20 | Ordered object probabilities and fixed profiles selected. | 50% nominal eligibility; conditional profile weights 12.5% BoltAction / 87.5% Regular; reroll equipment within the retained profile at respawn. |
| 2026-10-20 | Native building classes and optic thresholds inspected without launching DayZ. | SV98 qualifies for configured sniper-prone behavior; Scout 4x does not. Placement, lifecycle, combat, and host performance remain runtime acceptance work. |

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
