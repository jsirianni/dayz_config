# Deer Isle AI population

Updated: 2026-10-25

The previous 130 coordinate patrols and 24 legacy AI loadouts have been replaced
with 84 named definitions. Seven stable role selectors now choose among 32 named
archetypes, with additional clothing, bag, weapon and cargo choices in each kit.
There are 39 AI loadout files and three preserved player loadouts. Forty-three civilian
definitions follow native player-spawn centers; 41 object definitions attach the
other populations to their corresponding buildings and police-car event objects.
The three player loadouts, native spawn configuration, disabled Expansion spawn
selection, 58 roaming locations and disabled Expansion quest system are preserved.

## Population and relationships

| Category | Definitions | Faction | AI per patrol | Patrol cap | Living AI cap | Accuracy min-max |
| --- | ---: | --- | ---: | ---: | ---: | --- |
| Civilian | 43 | Civilian | 1 | 6 | 6 | 0.03-0.15 |
| PoliceStation | 4 | Guards | 2 | 3 | 6 | 0.08-0.25 |
| PoliceCar | 2 | Guards | 2 | 1 | 2 | 0.08-0.25 |
| Firefighter | 1 | Brawlers | 2 | 3 | 6 | 0.03-0.15 |
| Doctor | 3 | Civilian | 1 | 3 | 3 | 0.03-0.15 |
| Medic | 2 | Civilian | 1 | 3 | 3 | 0.03-0.15 |
| NATO | 23 | East | 3 | 6 | 18 | 0.10-0.35 |
| Militia | 6 | Mercenaries | 2 | 8 | 16 | 0.05-0.20 |
| **Total** | **84** | | | **33** | **60** | |

All category budgets cover 0-255 connected players. `Global.MaxPatrols` is 33;
Expansion counts patrols, not individual AI. Unused category capacity remains
reserved. These are ceilings, not guarantees that every category will fill or
every eligible building will have AI. Object eligibility is selected during
initialization; respawning does not give an ineligible object a fresh chance.

Players are explicitly assigned West on connect. NATO uses stock East for hostile
relationships with players; its equipment is NATO themed. Militia uses stock
Mercenaries: separate militia groups cooperate and fight players, NATO and other
hostile populations. Militia can attack Civilian medical staff and civilians.
The Civilian faction is not an immunity or an exact first-hit restraint policy.

Police use stock Guards, including conditional player aggression when threatened
and guard aggression memory. Firefighters use stock Brawlers, including its
10x melee damage multiplier and knockback behavior. This can make melee very
dangerous despite low firearm accuracy. Recruitment is disabled. There is no
custom faction mod, medic healing script, invulnerability or headshot protection.

Accuracy values are engine configuration parameters, not measured hit percentages.
Every role overrides the stock 0.35-0.95 accuracy range with lower values. Global
fallback accuracy is 0.05-0.25.

## Spawn sources

Civilian names `DI_Civilian_Spawn01` through `Spawn43` preserve source order and
the exact X/Z coordinates in `cfgplayerspawnpoints.xml`'s fresh-player generator
centers. Y=0 delegates terrain height to Expansion. Spread is zero. These centers
are anchors, not a claim that each center is a cleared final engine spawn point.
The player generator itself searches a grid around its centers.

Object patrol names contain their category and exact class name. Their waypoints
are empty and their `ObjectClassName` identifies the spawner. The inventory in
`../validation/ai-overhaul-object-classes.csv` records each binding and provenance:

| Population | Object classes | Listed static placements | Eligibility |
| --- | ---: | ---: | ---: |
| Police stations | 4 | 33 | 1.0 |
| Police cars | 2 | 1 matching map-listed object, plus dynamic event objects | 1.0 |
| Fire stations | 1 | 20 | 1.0 |
| Doctors: hospital/clinics | 3 | 34 | 1.0 |
| Medics: tents/field hospital | 2 | 33 | 1.0 |
| NATO military buildings | 23 | 408 | 0.25 |
| Militia apartments | 6 | 228 | 0.35 |

Police stations include both city and village stations and their Enoch variants.
Police cars use `Land_Wreck_sed01_aban1_police` and
`Land_Wreck_sed01_aban2_police`, the two children of the active `StaticPoliceCar`
event. That event has 35 potential positions and nominal population six. Matching
objects registered by Expansion receive this policy; it does not filter objects
by their creation provenance. Decorative Volha and hatchback police wreck classes
are not bound. The existing infected population attached to the event is unchanged.

Doctors bind `Land_City_Hospital`, `Land_Village_HealthCare` and
`Land_Village_HealthCare_Enoch`. Medics bind `Land_Medical_Tent_Big` and
`Land_WF_field_hospital_WEST_ep1`.

NATO uses `Land_Barrack_ep1`, `Land_Mil_Airfield_HQ`, both military ATCs,
`Land_Mil_Barracks_Round`, barracks 1-6, military tents Big1_1-5, Big2_1-2,
Big3-4, and `Land_Tisy_Barracks`, `Land_Tisy_Garages3`, `Land_Tisy_HQ`.
Each selected prototype has Military loot usage. Towers, radar platforms and
underground objects are excluded from this inventory.

Militia bind all six tenement variants: Big, Medium, Small, Small_Brick,
Small_Brown and Small_Yellow. Each class has exactly one profile; no ordered
multi-profile chance chain is needed.

All static placement counts come from the checked-in `mapgrouppos.xml`. They
are not a complete runtime world inventory or proof of safe ground/navigation.
No dedicated prison, ice-island or other quest-route patrols are added in this
phase; ordinary matching objects may still create incidental presence there.

## Movement, spawning and combat

Every patrol uses `ROAMING_LOCAL`: it selects buildings around the nearest roaming
location, rather than enforcing a perimeter around its originating object.
The preserved 58 roaming locations remain the map's location inventory.
Combat and local roaming can carry AI outside the original site.

- Activation: 200-900 m from players; despawn radius: 1,100 m.
- Despawn delay: 300 seconds; respawn delay: 1,800 seconds.
- Object waypoint spread: 20-40 m; police-car spread: 10-20 m; civilians: zero.
- Civilians/doctors/medics walk; other roles jog; all sprint under threat.
- NATO uses Vee formation; other roles use File. Scale is 1.5 and looseness zero.
- Group sizes are exact (`NumberOfAIMax: 0`); unit identities are engine selected.
- Persistence, AI triggering and contaminated-area spawning are disabled.
- Corpses are lootable. Damage multipliers are 1.0; headshot resistance is zero.
- Shoryuken chance and sniper-prone threshold are zero. Brawlers' separate native
  melee bonus remains active.

| Population | Threat distance | Noise investigation | Flanking distance | Flank outside combat |
| --- | ---: | ---: | ---: | --- |
| Civilian, doctor, medic | 100 m | 50 m | 25 m | No |
| Firefighter | 100 m | 50 m | 50 m | No |
| Police | 200 m | 100 m | 75 m | No |
| NATO | 300 m | 150 m | 150 m | Yes |
| Militia | 200 m | 100 m | 100 m | No |

Global fallback distances are 200/100/100 m, with outside-combat flanking off.
`UnlimitedReload: 6` permits unlimited reload only against animals and infected,
subject to Expansion's spare-magazine/ammunition requirements. Combat against
players can exhaust ammunition. Unlimited reload and scavenging mean later corpse
inventories may differ from the original kits.

## Starting equipment variety

Every AI independently runs its role selector, including members of the same
patrol. Repeated kits and multiple specialists in one patrol remain possible.
Archetype names describe equipment; they add no scripted squad roles or healing.
The patrols and inactive quest templates retain their existing seven `DI_*`
references. All 39 required AI filenames are in `../validation/ai-variety-files.txt`.

Each selector has 20 `ARCHETYPE` tickets, all with `Chance: 1`, and one `Include`
per ticket. Only one ticket with that group name is applied. For example, two
Marksman tickets out of 20 give a 10% per-AI chance. `Chance` is an eligibility
roll, not a normalized weight; replacing ticket counts with fractional chances
changes the resulting distribution. Optional bags, masks and headwear use explicit
empty tickets. Independent equipment groups own separate slots, and uniform sets
pair corresponding shirt/pants/hat colors. All alternatives are reevaluated per AI.

| Selector | Archetypes and selection weights | Distinct visible combinations, at least |
| --- | --- | ---: |
| `DI_Civilian` | Resident 25%, Worker 20%, Commuter 20%, Hiker 15%, Traveller 10%, Forager 10% | 56,520 |
| `DI_Police` | Patrol 45%, Station 25%, Traffic 20%, Response 10% | 918 |
| `DI_Firefighter` | FireCrew 50%, RescueCrew 35%, StationCrew 15% | 1,260 |
| `DI_Medic` | Ambulance 40%, Trauma 25%, Field 20%, Supply 15% | 144 |
| `DI_Doctor` | General 40%, Emergency 30%, Surgeon 20%, Clinic 10% | 72 |
| `DI_NATO` | Rifleman 40%, Pointman 15%, FieldMedic 15%, Engineer 10%, TeamLeader 10%, Marksman 10% | 4,752 |
| `DI_Militia` | Hunter 30%, Homesteader 25%, Scavenger 20%, Poacher 15%, Veteran 10% | 8,300 |

These counts deduplicate complete humanoid-slot appearances across archetypes.
They exclude health, quantities, cargo, weapon furniture and optics, so they are
conservative static counts. They assume compatible items can be created; they
are not observed frequencies or a guarantee of engine packing success.

### Role identity and bags

Civilian archetypes use hoodies/jeans, shirts/cargo or canvas pants, hiking jackets,
and raincoats in cataloged native colors. Forager clothing stays brown/green.
Sneakers and athletic shoes vary; headwear is absent 50%, with black, brown,
grey, blue and green beanies each 10%. Civilians start unarmed and unarmored.

Police keep paired standard/Orel uniforms, PoliceVest, a powered stun baton and
one bandage. Station selects standard/Orel equally; Response uses Orel; Patrol
and Traffic use standard. Traffic always has the police motorcycle helmet;
other kits select it or PoliceCap equally. Working/combat boots and gloves vary.

FireCrew wears black, RescueCrew beige, and StationCrew selects either paired
firefighter uniform. Helmets vary yellow/red/white, with working boots/gloves and
ReflexVest throughout. FireCrew selects one of three firefighter axe colors;
RescueCrew selects axe/wood axe/crowbar; StationCrew selects axe/crowbar equally.
The existing Brawlers bonus applies to these melee kits.

Ambulance, Trauma and Field medics wear crimson, blue and green paramedic outfits;
Supply selects among all three. Medics have a surgical mask 50% of the time.
General/Clinic doctors wear white scrubs, Emergency blue and Surgeon green, with
matching hats and surgical masks. Both roles vary surgical gloves and
black/brown/grey athletic shoes. They start unarmed and unarmored.

Militia vary hunting outfits, checked work shirts, hoodies and hiking jackets with
earth-tone pants, beanies/boonie hats and working/combat boots. Hunting patterns
stay paired. No military armor, automatic rifles or starting optics are supplied.

| Role | Bag probabilities |
| --- | --- |
| Civilian | None 50%; black/brown/grey sling bags 10% each; blue/green Taloon bags 10% each |
| Police | None 70%; black sling bag 15%; green small duffel 15% |
| Firefighter | None 60%; orange drysack 20%; green small duffel 20% |
| Medic/Doctor | Medical small duffel or medical canvas bag 50% each; doctors also keep medical hip pack |
| NATO | Green/black assault, green/brown Coyote, hunting or green Alice bag, equally |
| Militia | None 40%; brown/black sling, hunting or green Taloon bag 15% each |

### NATO weapons and protection

All NATO keep woodland USMC shirts/pants. Boots, tactical gloves and Mich/green
ballistic/woodland ballistic helmets vary. Standard kits select olive, khaki or
camouflaged UK assault vests equally. Marksman alone selects green/camouflaged
plate carriers with matching pouches. Armor can increase survivability despite
unchanged accuracy and damage multipliers.

| Archetype | Equal weapon choices | Sights |
| --- | --- | --- |
| Rifleman | M4A1, M16A2, Aug, FAMAS | Standard policy below |
| Pointman | MP5K, UMP45, Aug | Iron sights |
| FieldMedic | M4A1, M16A2, Aug | Standard policy below |
| Engineer | M4A1, FAMAS, UMP45 | Standard policy below |
| TeamLeader | M4A1, Aug | Powered M68 or Reflex equally |
| Marksman | FAL, M14 (native DMR classname) | 4x ACOG |

`AugShort` was excluded because its fixed scope conflicts with the requested
standard iron-sight policy. Duplicate Aug alternatives were merged; Rifleman
therefore has four equally likely weapon models. Standard Aug uses BUIS iron
sights. Standard M4 selects carry-handle iron sights 50%, M68 25%, Reflex 25%.
M4 furniture selects plastic/OE, RIS/CQB or MP/MP pairs equally. MP5 has its
standard stock and plastic handguard; FAL has its standard OE stock.

Across NATO, weapon probabilities are Aug 25%, M4A1 23.333333%, M16A2 15%,
FAMAS 13.333333%, UMP45 8.333333%, and MP5K/FAL/M14 5% each. Exact fractions
are recorded in the audit. Plate carriers and added magnified optics each occur
on the same 10% Marksman branch. No suppressors, NVGs, explosives, drum magazines
or secondary firearms are supplied.

### Police and militia weapons

| Archetype | Equal weapon choices |
| --- | --- |
| Police Patrol | Glock19, CZ75, MP5K |
| Police Station/Response | MP5K, UMP45, Mp133Shotgun |
| Police Traffic | Glock19, CZ75, FNX45 |
| Militia Hunter | CZ527, Winchester70, Mosin9130 |
| Militia Homesteader | Izh18, Izh18Shotgun, Izh43Shotgun, Repeater |
| Militia Scavenger | Ruger1022, Izh18Shotgun, Mp133Shotgun |
| Militia Poacher | CZ527, Mosin9130, SKS |
| Militia Veteran | SKS, Mosin9130, Mp133Shotgun |

Police/militia use iron sights; shotguns use buckshot. Every magazine-fed weapon
receives one full attached magazine and one full matching spare. Internally
loaded rifles/shotguns receive two full native ammunition stacks. Each exclusive
`WEAPON` branch contains its own firearm and matching ammunition. Spare ammunition
and all utility cargo are queued at the set/root level and packed after clothing
and bags have been created. Immediate chamber/loading readiness is unverified.

### Mandatory supplies and utility choices

Civilian, police, firefighter, NATO and militia kits each retain one bandage.
Both medical roles retain two bandages, one Terje hemostatic bandage and one
saline IV. Every doctor also retains a surgical kit, Flemoclav and Novacaine
ampoules, two empty syringes and quarter-full tetracycline. Doctor specialization
does not remove any of these supplies.

Each AI receives one extra equally selected utility bundle:

| Role/archetype | Alternatives |
| --- | --- |
| Civilian | Tuna, cola, matchbox, sewing kit, compass, heatpack |
| Police | Handcuffs with keys, powered personal radio, powered flashlight |
| Firefighter | Duct tape, heatpack, powered flashlight, extra bandage |
| Medic | Extra hemostatic bandage, disinfectant alcohol, heatpack, Terje plaster |
| Doctor | Stethoscope, vitamins, disinfectant alcohol, Terje agent test |
| Militia | Compass, can opener, matchbox, duct tape, weapon cleaning kit, tuna |
| NATO Rifleman | Tuna, heatpack |
| NATO Pointman | Powered flashlight, duct tape |
| NATO FieldMedic | Hemostatic bandage, saline IV |
| NATO Engineer | Weapon cleaning kit, electronic repair kit |
| NATO TeamLeader | Powered personal radio, binoculars |
| NATO Marksman | Powered rangefinder, binoculars |

All non-firearm items use 0.70-1.00 health; firearms use 0.60-0.90. Consumables,
batteries, magazines and ammunition are full (normalized quantity 1.0), except
doctor tetracycline at 0.25. The audit checks 232 native/Terje item classes and
all 45 archetype/weapon branches, representing 21 unique firearm models.
Class membership does not establish engine compatibility or medical cargo fit.

Civilians and medical staff can scavenge firearms, melee weapons and bandages,
including weapon upgrades. They retain their starting clothing. Firefighters can
upgrade melee weapons and find bandages; firearms and clothing scavenging are
disabled for them. Police, NATO and militia scavenge bandages only. The named roles
describe starting equipment, not a guarantee of permanently unarmed civilians.

## Validation and maintenance

From the repository root:

```powershell
.\skills\dzcli\scripts\install-dzcli.ps1
$dz = Join-Path (Get-Location) '.tools\dzcli\dzcli.exe'
& $dz validate expansion ai 'servers/first-light-deer-isle-expansion-us-east1'
& $dz validate expansion ai 'servers/first-light-deer-isle-expansion-us-east1/empty.deerisle'
python servers/first-light-deer-isle-expansion-us-east1/scripts/audit-ai.py
python -m unittest discover -s servers/first-light-deer-isle-expansion-us-east1/scripts -p 'test_*.py'
```

The audit is read-only by default and uses only Python's standard library.
`--write-report` refreshes the audit JSON and object-class CSV under `validation/`.
It verifies patrol policy, category arithmetic, all 45 weapon branches, full supply
quantities, catalog membership, JSON parsing, reference reachability/cycles,
exclusive slot ownership, paired uniforms, exact selection probabilities and
mandatory items on every branch. It also checks civilian coordinates, object
provenance and 13 preserved-file hashes, including the complete patrol file,
global AI settings, three player kits and four quest templates. The probability
analysis assumes successful compatible native item creation. Tests cover weighting,
empty alternatives, correlated uniforms, ammo branches, slot conflicts and includes.
Revise preservation baselines deliberately when future work changes those files.

Baseline/final dzcli output, combined `ai-overhaul-audit.json`, equipment-specific
`ai-variety-audit.json`, class-to-archetype inventory CSV, required AI filename list,
runtime checklist and preservation hashes are recorded under `../validation/`.
Original overhaul evidence is retained; `ai-variety-*` records this equipment pass.
Supported mutations were previewed and applied through dzcli, comparing previews
with written JSON. The variety pass does not edit patrols, global settings or
quest templates. Schema versions remain patrol 32, AI 20 and locations 4.
Validate using the server root to resolve sibling `Include` files; the CLI's
isolated-file validator lacks that sibling context and reports missing includes.

The inactive NPC templates now use `DI_Doctor`; the inactive bandit objective uses
`DI_Militia`; the inactive survivor escort uses `DI_Civilian`. These changes only
resolve references after old files are removed; they do not enable Expansion quests
or modify the map's own questline.

## Deployment and rollback

No live deployment or server launch was performed for this repository change.
Use the existing launcher and mod list; no new mod is required.

For an installation already running the 84-patrol overhaul, this equipment update
requires only the complete 39-file AI loadout set in `ai-variety-files.txt`.
Stop the server, back up the entire deployed loadout directory, copy every listed
file together, validate using both server and mission roots, then restart and
inspect logs. Retain the previous directory and configuration revision for rollback.
The selector names stay stable, but copying only selectors or only archetypes
creates an incomplete release. Keep all player kits and persistence untouched.

For deployment of the complete initial overhaul as well:

1. Stop the server and back up its mission `expansion/settings` folder, profile
   `ExpansionMod/Settings/AISettings.json`, complete `ExpansionMod/Loadouts` folder,
   and the four affected quest templates. Retain the backup with this revision ID.
2. Copy the new `empty.deerisle/expansion/settings/AIPatrolSettings.json` into the
   deployed mission, normally `C:\DayZ\server\mpmissions\empty.deerisle`.
3. Copy global `AISettings.json` and all 39 `DI_*.json` loadouts into the deployed
   profile, normally `C:\DayZ\server\profiles\ExpansionMod`. Keep all three existing
   player loadouts unchanged.
4. Copy the retargeted NPC 1/2, AI patrol objective 1 and AI escort objective 1 files
   to their matching profile quest paths. Keep `EnableQuests: 0`.
5. Remove only the 24 legacy AI loadout filenames enumerated in
   `../validation/ai-overhaul-removed-loadouts.txt`. Perform this as a complete
   stopped-server replacement so old patrols cannot run against missing kits.
6. Validate the deployed server and mission roots, then restart and review
   Expansion logs for missing classes, item creation errors and patrol counts.
   Preserve world, player and group persistence; no wipe is required.

Existing persisted player groups can retain an older faction despite the connect
default. Check existing characters as well as fresh characters. If an old group
has a different faction, correct that group using the server's normal admin/group
workflow after backing it up; do not wipe characters to fix relationships.

For equipment-only rollback, stop the server and restore the previous complete
loadout directory. Remove the 32 archetype files introduced by this version if
restoring the seven-kit version; use the filename manifest and backup to identify
the exact set. Restore the matching audit/documentation revision for repeatable
checks. Do not delete player loadouts or player/world/group storage.

For full-overhaul rollback, stop the server, restore the backed-up patrol settings, global AI
settings, four templates and complete loadout directory together, validate, then
restart. Remove all 39 DI files when restoring the original legacy loadout directory.
Keep player/world persistence intact. Retain the new files separately if needed
for diagnosis. Recheck the custom mission overlay after game or map updates.

## Runtime acceptance still required

- Use `../validation/ai-variety-runtime-checklist.md` to exercise all 32 archetypes
  and 45 weapon branches (21 firearm models). Check attached/spare magazines, loose
  ammo, loading/firing readiness, M4 furniture, MP5/FAL stocks and optic compatibility.
- Verify BUIS on standard Aug, powered M68/Reflex and utility devices, 4x ACOG
  and matching plate pouches only on Marksman, and absence of AugShort/NVGs/suppressors.
- Inspect each medic/doctor bag option for packing failures and missing mandatory
  supplies, including doctor hip pack/syringes/ampoules/tetracycline. Repeat with
  every utility option and the smallest containers. These NPCs do not heal players.
- Observe natural multi-AI patrols and respawns for independent equipment rolls,
  coherent uniforms, useful cargo and readable role silhouettes. Repetition and
  multiple Marksmen in one patrol are possible; 10% is not a squad quota.
- Check West players against East NATO and Mercenaries militia, cooperation within
  each armed role, conditional police aggression, Brawlers melee and neutral-role
  responses. Include existing player groups and reconnects.
- Observe civilian/medical weapon scavenging, unchanged clothing, firefighter
  melee-only scavenging, and bandage-only scavenging for police/NATO/militia.
- Test ammunition exhaustion against players and permitted refills against infected
  and animals. Scavenged weapons still need usable ammunition/magazines.
- Visit each object-class family and player-spawn region: inspect terrain height,
  spread, collision clearance, access off the spawn surface, formation placement
  and local roaming. Listed objects are not prevalidated spawn geometry.
- Observe activation/despawn/respawn behavior, restart behavior, replacement/removal
  of dynamic police wrecks and absence of orphaned patrol spawners.
- Check per-category and global limits at low and high player counts and measure
  server performance. Sixty living AI is a configured ceiling, not a benchmark.

## Upstream behavior references

- [Patrol settings, factions, reload mask and local roaming](https://github.com/salutesh/DayZ-Expansion-Scripts/wiki/%5BServer-Hosting%5D-How-to-create-AI-Patrols)
- [Load balancing](https://github.com/salutesh/DayZ-Expansion-Scripts/wiki/%5BServer-Hosting%5D-AI-Load-Balancing)
- [Stock global defaults](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/release/DayZExpansion/AI/Scripts/3_Game/DayZExpansion_AI/Settings/ExpansionAISettings.c)
- [Stock Brawlers](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/release/DayZExpansion/AI/Scripts/3_Game/DayZExpansion_AI/Factions/eaifactionbrawlers.c)
- [Stock Mercenaries](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/release/DayZExpansion/AI/Scripts/3_Game/DayZExpansion_AI/Factions/eAIFactionMercenaries.c)
- [Native loadout selection and includes](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/release/DayZExpansion/Core/Scripts/4_World/DayZExpansion_Core/Classes/Prefab/ExpansionPrefabObject.c)
- [AUR fixed-scope versus modular variants](https://dayz.com/article/game-update/DayZ-Update-1-15)
- [Object profile selection](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/release/DayZExpansion/AI/Scripts/4_World/DayZExpansion_AI/Classes/Patrols/PatrolManager.c)

Upstream branch links can change; the deployed mod/build must be checked during
runtime acceptance. This audit establishes configuration consistency only.
