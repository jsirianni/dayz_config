# First Light Badlands Nasdara Expansion AI

Public Nasdara configuration with CF, VPPAdminTools, and Expansion AI in
`first-light-badlands-nasdara-staging-us-east`, copied from
`../first-light-badlands-nasdara-us-east/` at main commit `5c6306f`.
`../first-light-frostline-sakhal-us-east/` is the read-only configuration reference.
The mission directory contains only custom overrides for a complete installed
stock Nasdara mission.

PR #58's AI overlay was merged to main. The operator reports that staging is
deployed on its own VM, starts successfully, and shows the correct DZSA mod
list. The assasin update described below is configuration-only and
has not been deployed or verified in game by this change. This configuration
is promoted to full-time public use; the separate original Nasdara
configuration is untouched.

## Server settings and ports

| Setting | Value |
| --- | --- |
| Public hostname | `First Light \| Badlands Nasdara \| Expansion AI \| us-east1` |
| Join password | None (`password = ""`); public joining is enabled |
| Mission template | `dayzOffline.nasdara` |
| Game port | `2901` (`-port=2901`) |
| Client port | `2903` (`clientPort = 2903`) |
| Steam query port | `27018` (`steamQueryPort = 27018`) |
| RCon port | Unspecified, matching Frostline; runtime/default value unverified |
| Maximum players | 60 |
| Player view | First person; crosshair disabled |
| Time | System time, 2x day acceleration, 5x night multiplier, persistent |
| Mods | CF, VPPAdminTools, Dabs Framework, Expansion Core, Expansion AI (in that order) |

The remaining server configuration settings mirror the production Nasdara server. The Windows
launchers retain logging and BattlEye options, with profiles beneath the Windows
installation directory. VPP's
`vppDisablePassword` field remains omitted, so VPP password authentication is enabled.

The configured game, client, and query ports are confirmed from repository files;
live listeners and router/firewall rules require checking on the host. RCon is
unchanged. Public joining does not change DayZ admin or VPP credentials.
Deer Isle vanilla also uses game port 2901 and query port 27018; these servers
need separate host/IP bindings when run concurrently.

## Public promotion and player spawn gear (2026-10-20)

The advertised hostname replaces `STAGING` with `Expansion AI`. The existing
repository directory keeps its deployment path. Joining requires no password;
admin/VPP authentication, ports, mods and all AI configuration are unchanged.

`mpmissions/dayzOffline.nasdara/init.c` is based on the pinned official Nasdara
initializer. It preserves the stock economy/date initialization, random regional
shirt/pants/sandals, chemlight, soda, figs, one bandage and starting water/energy.
Only fresh characters receive the additions below; reconnecting with an existing
character does not reroll gear.

Every row has its own **independent 5% chance**, with at most one selection from
that row. Multiple rows can succeed, or none can succeed. Item variants within
a row have equal probability; this is not 5% per variant.

| Bonus category | Selection / contents |
| --- | --- |
| Water container | `WaterBottle`, `GlassBottle`, `Canteen`, `FilteringBottle`, `SmallGlassBottle_Afza`, `SmallGlassBottle_Jallab`, `WineBottle`, `AyranBottle`; filled with clean water rather than native drink contents |
| .380 pistol | `MakarovIJ70` with one full attached `Mag_IJ70_8Rnd` and a chambered round; no loose .380 ammunition or spare magazine |
| Badlands vest | Six Linen Vest colors, five Robe Vest colors, or three Chest Rig colors |
| Kitchen knife | `KitchenKnife` |
| Goggles | Two Dust Goggles, Tactical Goggles, or three Ski Goggles variants; no night vision |
| Face mask | Eight Keffiyeh colors, beige/black/green Balaclava Masks, Surgical Mask or NIOSH Face Mask |
| Single-barrel shotgun | `Izh18Shotgun` (unloaded) plus one full `Ammo_12gaPellets` stack; stack size uses the item's native maximum |
| Low-tier backpack | `CourierBag` or blue/green/orange/violet `TaloonBag` |
| Medical hip pack | `HipPack_Medical`, with one extra `BandageDressing` placed inside where possible, in addition to the stock bandage |
| Road flare | `Roadflare` |
| Compass | `Compass` or `OrienteeringCompass` |
| Lee-Enfield | `LeeEnfield` only; no magazine or ammunition added |

Bonus gear starts pristine. Storage rolls run first. Successful item rolls try
inventory, then hands, then placement on the ground at the player if space is
exhausted. The medical bandage tries the hip pack first. Creation failures log a
`[SpawnGear]` message; no additional chance rolls or replacement items are used.

Stop the server, back up the deployed initializer, then copy this directory's
`serverDZ.cfg` and `mpmissions/dayzOffline.nasdara/init.c` over the matching files
under the DayZ installation. Retain the host's other stock mission files,
profiles, AI settings/loadouts and persistence. Restart using the existing
launcher. No profile or player-data wipe is needed.

Baseline stock initialization and final custom initialization pass `dzcli
validate init`; the server configuration passes before and after. All explicit
inventory classes were checked against pinned official Nasdara `types.xml`.
See [promotion validation](validation/public-promotion.txt) and the
[static audit](validation/public-promotion-audit.json). These are configuration
checks, not an Enforce compilation or in-game verification. After deployment,
confirm the public name/join access, new-character gear, loaded IJ-70, complete
buckshot stack, usable water, equipment slots, and inventory-overflow handling.

Sources: [stock initializer](https://github.com/BohemiaInteractive/DayZ-Central-Economy/blob/f220b2918b1d79a5055b1ee9be70c0ec1516e876/dayzOffline.nasdara/init.c),
[official item classes](https://github.com/BohemiaInteractive/DayZ-Central-Economy/blob/f220b2918b1d79a5055b1ee9be70c0ec1516e876/dayzOffline.nasdara/db/types.xml),
and [official weapon-loading API](https://github.com/BohemiaInteractive/DayZ-Script-Diff/blob/e3dda95b2b9730bdc0dba931abb35304b484bbfc/scripts/4_world/entities/firearms/weapon_base.c).

## Mission override and provenance

The mission overlay includes `mpmissions/dayzOffline.nasdara/db/messages.xml` and
`mpmissions/dayzOffline.nasdara/expansion/settings/AIPatrolSettings.json`, plus a customized
`mpmissions/dayzOffline.nasdara/init.c`.
The messages retain Frostline's server-name greeting and Discord invite, and add
a graceful shutdown countdown. Provision the full stock mission on the host, then
copy messages into its existing `db` directory, patrol settings into its
`expansion/settings` directory, and the customized `init.c` into the mission
root. Preserve the remaining stock mission files.

The original 51-file mission import came from
[BohemiaInteractive/DayZ-Central-Economy](https://github.com/BohemiaInteractive/DayZ-Central-Economy/tree/f220b2918b1d79a5055b1ee9be70c0ec1516e876/dayzOffline.nasdara).

- Upstream commit: `f220b2918b1d79a5055b1ee9be70c0ec1516e876`.
- Pristine import commit: `d206bc5`.
- Pristine mission Git tree: `1644633799219b52320e9220ead3fa1c8e1e90d3`.

The pristine import preserved every upstream path, file mode, and byte. A
comparison of Git blobs and modes confirmed that the other 50 files were still
unchanged stock files. They have been removed from this directory, including
`areaflags.map`, the economy databases, environment territories, map placement
data, stock XML/JSON settings, weather, and the then-stock `init.c`. This public-server
update restores `init.c` as a customized override. The installed stock mission
supplies the remaining files. The original complete import remains in Git history.

The mission's `db/messages.xml` uses DayZ's built-in graceful shutdown countdown,
with `<deadline>180</deadline>` and `<shutdown>1</shutdown>`.
The deadline is three hours of server uptime and resets on each start; it is not
a fixed wall-clock schedule. DayZ broadcasts warnings at 90, 60, 45, 30, 20, 15,
10, 5, 2, and 1 minute remaining, then shuts down gracefully. The old 1 PM/4 AM
EST announcement has been replaced by the countdown message. See the
[official server-message documentation](https://community.bistudio.com/wiki/DayZ:Server_Messages).

Run `dayz-service-manager.bat` to complete the restart cycle: after DayZ exits,
the manager waits 10 seconds, updates the server/mods, and launches it again.
Update/download time adds to downtime. Running `dayz-service.bat` alone leaves
the server stopped after the countdown. Deploy the updated messages into
`C:\DayZ\server\mpmissions\dayzOffline.nasdara\db\messages.xml` before starting
the server, and verify warnings, shutdown, persistence, and relaunch on the host.
No BEC, RCon scheduler, or Windows scheduled task is required.

## Base mods and admin access

`dayz-service.bat` installs/updates these official Workshop mods through SteamCMD's
DayZ application `221100`:

| Load order | Mod | Workshop ID |
| --- | --- | --- |
| 1 | [CF (Community Framework)](https://steamcommunity.com/sharedfiles/filedetails/?id=1559212036) | `1559212036` |
| 2 | [VPPAdminTools](https://steamcommunity.com/sharedfiles/filedetails/?id=1828439124) | `1828439124` |
| 3 | [Dabs Framework](https://steamcommunity.com/sharedfiles/filedetails/?id=2545327648) | `2545327648` |
| 4 | [DayZ-Expansion-Core](https://steamcommunity.com/sharedfiles/filedetails/?id=2291785308) | `2291785308` |
| 5 | [DayZ-Expansion-AI](https://steamcommunity.com/sharedfiles/filedetails/?id=2792982069) | `2792982069` |

VPP requires CF on both the server and connecting clients. The Windows launcher
mirrors the downloaded mods into the five matching `@mod` folders beneath the install
directory, copies signing files into `keys` using the reference script's key
patterns, and loads
`-mod=@CF;@VPPAdminTools;@DabsFramework;@DayZ-Expansion-Core;@DayZ-Expansion-AI`.
The synchronization helper and
error handling match the Deer Isle batch reference.
Clients must install compatible versions of all five mods and load them in the
same order. See the
[VPP installation guide](https://github.com/VanillaPlusPlus/VPP-Admin-Tools/wiki/Installation-%26-Configuration).

`profiles/VPPAdminTools/` copies all three files from Frostline byte for byte,
including the existing superadmin list and VPP credentials file. Only those
listed Steam64 identities receive superadmin access. Additional VPP settings
are generated by the mod on startup. No CodeLock or BuildAnywhere configuration
is included.

VPP's additional password prompt stays enabled. Use the VPP credential from the
copied `Permissions/credentials.txt`; this is separate from DayZ's `passwordAdmin`.
VPP hashes the deployed plaintext credential file on startup. Manage credential changes
on the host when deploying, then restart the server to
apply any changes. The existing profile readme describes Frostline's previous
password setup; Badlands requires the credential because password checks remain on.

CF's Workshop installation notes require clean persistence when first enabling
CF. If this server has already run, back up its existing persistence and plan
that change during deployment. This repository change does not delete storage.

## Expansion AI configuration

The overlay provides **45 loadout files**: four Civilian, nine Tribesman,
13 Warlord, 16 Western_Merc (including one MP5 medic), two Merc selectors, and
`Nasdara_Assasin.json`. There are **143 patrol definitions**: the original 40
single-AI Civilian/Tribesman entries at 20 official player spawn-area centers,
102 two-AI Merc object entries covering all 51 selected military-related
building/object classes, and one single-AI patrol named exactly `assasin`.
The player relationship phase changed the faction on 20
Tribesman patrols to Civilian. Players remain West, Mercs East, Civilians
Civilian, and assasin Raiders; future Warlords use stock Mercenaries.
Warlord patrols, original Civilian/Tribesman accuracy-tier overrides,
and the geometry scanner remain future work. Full equipment tables,
[assasin settings](AI_IMPLEMENTATION.md#assasin-patrol-update-2026-10-20), and
[player relationship policy](AI_IMPLEMENTATION.md#player-relationship-update-2026-10-20)
are in [AI_IMPLEMENTATION.md](AI_IMPLEMENTATION.md).

| Repository path | Deployment destination |
| --- | --- |
| `profiles/ExpansionMod/Loadouts/Nasdara_Civilian_01.json` through `04.json` | `<active-profile>/ExpansionMod/Loadouts/`, same filenames |
| `profiles/ExpansionMod/Loadouts/Nasdara_Tribesman_01.json` through `09.json` | `<active-profile>/ExpansionMod/Loadouts/`, same filenames |
| `profiles/ExpansionMod/Loadouts/Nasdara_Warlord_01.json` through `13.json` | `<active-profile>/ExpansionMod/Loadouts/`, same filenames |
| `profiles/ExpansionMod/Loadouts/Nasdara_Western_Merc_01.json` through `16.json` | `<active-profile>/ExpansionMod/Loadouts/`, same filenames; 11 is `Nasdara_Western_Merc_11_Medic.json` |
| `profiles/ExpansionMod/Loadouts/Nasdara_Western_Merc_BoltAction_Random.json` and `Nasdara_Western_Merc_Regular_Random.json` | `<active-profile>/ExpansionMod/Loadouts/`, both files required by the new patrols |
| `profiles/ExpansionMod/Loadouts/Nasdara_Assasin.json` | `<active-profile>/ExpansionMod/Loadouts/Nasdara_Assasin.json` |
| `profiles/ExpansionMod/Settings/AISettings.json` | `<active-profile>/ExpansionMod/Settings/AISettings.json` |
| `mpmissions/dayzOffline.nasdara/expansion/settings/AIPatrolSettings.json` | Same path beneath the DayZ installation |

For loadouts, copy the `ExpansionMod/Loadouts` portion beneath the selected
profile directory, e.g. `C:\DayZ\server\profiles\ExpansionMod\Loadouts\`.
Do not add another nested `profiles` directory. Patrol settings use schema 32;
AI settings use schema 20, inspected at Expansion commit
`0d26645ca31bc574bc3ebee1d4bc9192727f3c43`. Use a compatible deployed release.

### Starting equipment and patrol behavior

| Role | Starting clothing and equipment | Scavenging |
| --- | --- | --- |
| Civilian | Badlands Kameez/short-sleeve tops, Shalwar pants, regional hats/sandals/vests; one knife, hatchet or pipe | Disabled (`NONE`); corpse remains lootable |
| Tribesman | Badlands Kameez/Shalwar or olive OBR outfit, regional hats/sandals and chest rigs; original shotgun/Mosin/SKS definitions plus three .380 SMG additions | Disabled (`NONE`); existing patrols retain their original six loadouts |
| Warlord | Traditional regional clothing; probabilistic tactical/ballistic vest; Soviet weapons and regional additions, full magazines or limited loose ammunition | New loadout definitions only; patrol behavior deferred |
| Western_Merc | Matching USMC desert uniform, plate carrier/pouches, helmet, backpack; western firearms, optics and compatible suppressors, medical supplies | Firearms, melee, bandages, clothing, upgrades; object patrols use stock East |
| assasin | Desert ghillie suit/hood over USMC desert clothing; suppressed VSD with PSO6/wrap, two full magazines, rangefinder, bandage, full-water canteen | Bandages only; retains starting rifle and outfit |

All starting items are pristine. Explicit inventory class names use
the pinned official Nasdara
[types.xml](https://github.com/BohemiaInteractive/DayZ-Central-Economy/blob/f220b2918b1d79a5055b1ee9be70c0ec1516e876/dayzOffline.nasdara/db/types.xml).
The original ten loadouts choose headwear, footwear and vests uniformly;
Tribesman_06 has a fixed OBR cap. Some loadouts independently roll bottles,
bandages or matches; three have no helper supplies. Civilians start without
firearms/ammunition. Civilian and Tribesman patrols use `LootingBehaviour: NONE`
and do not scavenge weapons, bandages or clothing. Their corpses remain lootable;
unlimited reload can still change the ammunition left on a corpse.

The original six Tribesman loadouts start with one small compatible loose-ammo stack: normalized quantity
0.20 for shotgun shells, 0.25 for Mosin cartridges or 0.40 for SKS cartridges
(2 buckshot shells in a ten-shell stack, or 5/8 rifle rounds in 20-round stacks). `UnlimitedReload: 1` replenishes
ammunition during use and can leave more ammo on a corpse than initially given.
Bottles start at 35-65% capacity with native liquid defaults.

Civilian and Tribesman patrols both use stock `Civilian`, with scavenging
disabled (`LootingBehaviour: NONE`) and lootable corpses (`CanBeLooted: 1`). Both use
`ROAMING_LOCAL`, walk outside combat, and sprint under threat. Threat, noise investigation and
flanking distances are 45.72 m (50 yards), with flanking outside combat disabled.
Both roles have `Chance: 0.75` (75% at patrol initialization); this does not
guarantee a fixed active population. Tribesmen retain stock `Civilian`: there
is no stock Police faction, and `Guards` would change gameplay relationships.
See [the conditional Police decision](AI_IMPLEMENTATION.md#civilian-and-tribesman-75-spawn-chance-2026-10-20).
Activation distances are 45.72-500 m; despawn distance is 600 m, despawn time
300 seconds and respawn time 1800 seconds. Patrols are lootable and do not
persist across restarts. Their category caps remain 20 Civilian and 20 Tribesman
single-AI patrols for every player count from 0 through 255. The global cap is
now 61 patrols, with a separate 10-patrol Merc cap (two AI per patrol) and
`Assasin` cap 1. Category caps together permit at most **51 active patrols /
61 living AI**. The assasin uses persistence across restarts; existing patrols
retain their prior nonpersistent settings.
Unused category capacity is not borrowed by other roles.


| Role | Patrol cap | AI per patrol | Maximum living AI |
| --- | ---: | ---: | ---: |
| Civilian | 20 | 1 | 20 |
| Tribesman | 20 | 1 | 20 |
| Western_Merc | 10 | 2 | 20 |
| assasin | 1 | 1 | 1 |
| Total | **51** | | **61** |

The current size reduction sets `NumberOfAI: 1` on all 40 Civilian/Tribesman
definitions; `NumberOfAIMax` remains 0 for a fixed one AI per patrol. Scavenging
remains disabled. Coordinates, native spread and behavior are unchanged.
Verify single-AI spawning and ground clearance in staging. Deploy the mission patrol file with staging stopped and restart
using the existing launcher, retaining profiles, loadouts and persistence.

### Faction alliances (2026-10-20)

| Role | Stock Expansion faction | Baseline relationships |
| --- | --- | --- |
| Civilian | `Civilian` | Initially friendly to players, Tribesmen and Mercs; future Warlords can attack them |
| Tribesman | `Civilian` | Initially friendly to players, Civilians and Mercs; future Warlords can attack them |
| Warlord (future patrols) | `Mercenaries` | Cooperates across Warlord patrols; attacks players, Mercs, Civilians and Tribesmen |
| Western_Merc | `East` | Cooperates across Merc patrols; fights players and future Warlords; initially accepts Civilians/Tribesmen |
| Players | `West` | Initially friendly to Civilians/Tribesmen; hostile to Mercs and future Warlords |
| assasin | `Raiders` | Hostile to players and all implemented AI factions, including Civilians |

The user's Taliban reference means the existing Warlord role. Its 13 loadouts
are ready; patrol spawning remains deferred. This update changes only
`Faction` on the 20 Tribesman patrol definitions from `West` to `Civilian`.
It supersedes the previous West Tribesman/Warlord alliance: future Warlords
must use stock `Mercenaries`, making them enemies of players and East Mercs.
Merc patrols retain `East`; players retain `PlayerFactions: ["West"]`.
All 45 loadouts, other patrol fields, population limits and assasin settings
are preserved. No custom faction is required.

For this update, stop staging and replace only mission
`mpmissions/dayzOffline.nasdara/expansion/settings/AIPatrolSettings.json`, then
restart using the existing launcher. Retain profile `AISettings.json`,
loadouts and persistence. This update performs no deployment or data reset.

Friendly/guard recruitment stays disabled. Stock retaliation and friendly
assistance remain enabled and can draw Civilians into nearby battles; initial
friendship is not a guarantee they never fight. Existing persistent player-group
factions may need separate attention during deployment. No custom mod is required
for these baseline alliances. Runtime relationship checks remain pending.

### assasin patrol

A single persistent `assasin` uses stock `Raiders`, with category cap 1. Its
pristine guaranteed kit includes the VSD (`SVD`), `AK_Suppressor`, powered
`PSO6Optic`, `GhillieAtt_Desert`, one attached full ten-round magazine and one
full spare, desert ghillie suit/hood, USMC desert top/pants, beige boots/gloves,
tactical goggles, beige balaclava, powered rangefinder, bandage, and full-water
canteen. Bandage-only scavenging preserves its starting rifle and outfit.

| assasin setting | Value |
| --- | --- |
| Population / patrol name | One AI / `assasin` |
| Faction / category | `Raiders` / `Assasin` |
| Behavior / normal / threat speed | `ROAMING` / `JOG` / `SPRINT` |
| Accuracy | 0.85-0.95; selected settings, not measured hit rates |
| Threat / investigation / flanking range | 1000 / 1000 / 1000 m |
| Flanking outside combat | Enabled |
| Sniper-prone threshold | 600 m; qualifying optic and native conditions required |
| Minimum activation distance | 3000 m |
| Maximum activation / despawn distance | 25000 / 25000 m |
| Native spawn spread | 40-80 m |
| Despawn / respawn time | 300 / 1800 seconds |
| Lootable / unlimited reload / persistence | Enabled / enabled / enabled |
| Scavenging | `BANDAGES` only |

The starting waypoint is **`[12170.946289, 0, 2173.485107]`**, copied from a
`Land_Mausoleum_Small1_Dam1` placement in the pinned official CE file. Only two
placements of this class are listed; this selected one is **1727.05 m from the
nearest Merc object**. It is the most separated of 324 placements across 138
classes with one through five listed instances. The other same-class placement
is only 83.47 m from a Merc object and is excluded.

The [current manifest](validation/assasin-remote-spawn-points.csv) records
coordinates, source index and nearest-Merc distance. The former
[460-point manifest](validation/assasin-spawn-points.csv) remains historical.
See [selection and provenance](AI_IMPLEMENTATION.md#assasin-spawn-relocation-2026-10-20).
One anchor replaces the random choice among military objects; existing 40-80 m
native spread still varies initial placement. The 3000-25000 m player activation
ring still applies, and neither native elevation nor spread verifies safe ground.
The patrol remains coordinate-based and persistent; Merc object spawners are unchanged.

`ROAMING` chooses map named locations; it does not always know player positions.
The 25 km activation maximum/despawn radius aims to retain the patrol anywhere
on Nasdara while players are online. Persistence restores a living patrol's
position/equipment across restart instead of rerolling its spawn. Restoration
also obeys the 3000 m minimum and can wait if a player is too close. The spare
magazine supplies compatible ammunition for unlimited reload, which can leave
more than the initial 20 rounds on a corpse. In-game placement, inventory,
long-range detection, prone behavior, roaming and persistence remain unchecked.

### Western Merc object patrols

[validation/merc-object-classes.csv](validation/merc-object-classes.csv) records
the exact 51 selected classes, source order, counts, and pinned CE revision.
All selected classes are included, including fuel tanks. Official
[map placements](https://github.com/BohemiaInteractive/DayZ-Central-Economy/blob/f220b2918b1d79a5055b1ee9be70c0ec1516e876/dayzOffline.nasdara/mapgrouppos.xml)
list **460 objects** across those classes. Runtime spawner counts and safe ground
are unverified. Class matching is not restricted to named military bases.

Each class has an ordered pair of definitions with empty `Waypoints`:
`BoltAction` first (chance 0.0625), then `Regular` (chance 0.4666666666666667).
First-success selection gives nominal 50% object eligibility and conditional
12.5%/87.5% profile weights. BoltAction uniformly selects SV98 or Scout;
Regular uniformly selects the other 14 Merc kits. Each member rolls independently,
giving all 16 kits equal nominal weighting overall. Eligibility/profile selection
happens when the object spawner initializes; respawns retain the profile and
reroll equipment within it. Actual active kit counts need not be equal.

| Merc setting | Value |
| --- | --- |
| Group / category | 2 AI / `Western_Merc`, capped at 10 patrols |
| Faction / behavior | `East` / `ROAMING_LOCAL` |
| Normal / threat speed | `JOG` / `SPRINT` |
| Accuracy | 0.45-0.95; selected settings, not measured hit rates |
| Threat / noise / flanking range | 200 / 100 / 200 m |
| Flanking outside combat | Enabled; assasin also enables it |
| Sniper prone threshold | BoltAction 190 m; Regular 0 |
| Activation / despawn distance | 250-1000 / 1200 m |
| Native spawn spread | 40-80 m around the object |
| Despawn / respawn time | 300 / 1800 seconds |
| Lootable / unlimited reload | Enabled / enabled |
| Persistence / AI-triggered spawn / contaminated-area spawn | Disabled / disabled / disabled |

Merc scavenging allows firearms, melee weapons, bandages, clothing, and upgrades.
Scavenging never changes the assigned behavior profile. SV98's MK4 qualifies for
automatic sniper-prone shooting; Scout's required 4x ACOG does not. Facing the
target and no recent hit are also required. Regular profiles disable automatic
sniper-prone behavior; other prone cover behavior remains possible.

Empty waypoints and spread do not establish flat, outdoor, unobstructed ground.
Several structures in one base can generate several eligible spawners. Local
roaming can leave the compound. Stock lifecycle settings do not guarantee
continuous presence or exact wall-clock transitions. All 51 native class
definitions/inheritance were checked against installed build 25956772 without
launching DayZ; deployed-version parity still matters.

### Additional loadout inventory

The original ten loadout files are preserved. Tribesman_07/08/09 copy the approved
01/02/03 clothing and supplies with CZ61, RAK, or Bizon respectively, one full
attached magazine, no spares, and no loose ammunition. The Bizon includes its
buttstock. Existing patrol loadout assignments are unchanged.

Warlords have 13 weapon families, complete compatible furniture, no suppressors,
and only optional PU scopes on Mosin/SKS. They carry one full magazine plus one
or two spares (Bizon zero or one), or 20 loose rounds/shells for internal-magazine
weapons. Their vest roll is 30% none, 40% chest rig, 20% press vest, 10% plate
carrier with pouches. Gold KA-M is a 1% branch; only Bizon has a backpack.

Mercs have 16 firearm loadouts, two to four full spare magazines uniformly, or
40 loose buckshot shells for R12. Compatible removable optics roll at 90% and
suppressors at 60%; Scout always has 4x ACOG and SV98 always has its compatible
scope. Unsupported stock suppressors are excluded. SVAL has integral
suppression. NVGs roll at 1%. Batteries are full and attached to powered items.

Ordinary Mercs have one full bandage roll guaranteed and 18 optional medical
items, each independently at 25%. The MP5 medic has medical canvas backpack,
medical hip pack, white surgical gloves and surgical mask, with three full
bandage rolls, two IV kits, two empty blood bags, two saline bags, two epinephrine,
two morphine and one full 12-tablet tetra pack guaranteed. Twelve other medical
items each roll at 25%. Advanced healing behavior is not supplied by a loadout;
these are carried supplies and potential loot.

Every Warlord/Merc has one 5% food gate selecting at most one faction-appropriate
food uniformly: sealed halva/sarma for Warlords; tactical bacon, crackers or
chocolate for Mercs. No Civilian/Tribesman food is added. See the full plan for
utility probabilities, native quantities, conditional attachments, and complete
weapon/medical tables. Static capacity feasibility remains subject to native
engine acceptance.

### Placement and deployment prerequisites

The coordinates copy `fresh/generator_posbubbles/group/pos` in the official
[player spawn file](https://github.com/BohemiaInteractive/DayZ-Central-Economy/blob/f220b2918b1d79a5055b1ee9be70c0ec1516e876/dayzOffline.nasdara/cfgplayerspawnpoints.xml)
at CE commit `f220b2918b1d79a5055b1ee9be70c0ec1516e876`. The duplicate `hop` and
`travel` lists are excluded. X/Z precision and source order are preserved with
waypoint Y=0 for Expansion elevation resolution. Civilian native spread is
1-2 m; Tribesman spread is 4-5 m around the same anchor.

These are **unverified spawn-area centers**. Neither Y=0 nor native spread
establishes flat, outdoor, clear ground or safe elevation. The desired 500 m
roaming area is not a hard boundary: activation radius does not enforce a
movement leash, and stock local roaming can choose farther destinations.

The stock Expansion AI Workshop dependency list requires the following mods;
see the [official AI page](https://steamcommunity.com/sharedfiles/filedetails/?id=2792982069)
and [installation guide](https://steamcommunity.com/sharedfiles/filedetails/?id=2430317812).

| Dependency order | Mod | Workshop ID |
| --- | --- | --- |
| 1 | [CF](https://steamcommunity.com/sharedfiles/filedetails/?id=1559212036), already in the launcher | `1559212036` |
| 2 | [Dabs Framework](https://steamcommunity.com/sharedfiles/filedetails/?id=2545327648) | `2545327648` |
| 3 | [DayZ-Expansion-Core](https://steamcommunity.com/sharedfiles/filedetails/?id=2291785308) | `2291785308` |
| 4 | [DayZ-Expansion-AI](https://steamcommunity.com/sharedfiles/filedetails/?id=2792982069) | `2792982069` |

For deployment, install compatible versions on the server and connecting
clients, load dependencies before Expansion AI, and deploy their signing keys
following the official guide. Keep VPP's existing CF dependency. Copy this
server's `profiles/ExpansionMod` overlay to the active profile directory and
the patrol file to the installed stock Nasdara mission. Preserve installed
mission data and unrelated existing profile settings; review any existing AI
settings before replacing them. Stock roaming-location settings generated for
Nasdara still require runtime inspection.

The staging launcher installs/updates, synchronizes, and loads these AI
dependencies alongside CF/VPP, using the existing signing-key helper. The
operator reports a successful prior deployment/startup on a separate VM and
correct DZSA mod list. The launcher is unchanged by this object-patrol phase.
For this spawn relocation, stop staging and replace only the mission
`mpmissions/dayzOffline.nasdara/expansion/settings/AIPatrolSettings.json`, then
restart with the existing launcher. **A living persisted assasin retains its
saved position; restarting alone does not move it.** The new anchor takes effect
on its next fresh spawn after death/removal. Retain all profile data, loadouts
and persistence. This change does not reset persistence or deploy itself.

For the original assasin installation, stop staging and copy the updated
`AIPatrolSettings.json` into the existing mission `expansion/settings` directory.
Copy `Nasdara_Assasin.json` into the active profile's `ExpansionMod/Loadouts`
directory, retaining the other 44 files, then restart with the existing launcher.
Retain existing profile data, stock mission files and persistence. This update
does not add a custom mod or deploy itself.

### Configuration validation

Latest spawn-chance records are
[server baseline](validation/civilian-tribesman-chance-baseline.txt),
[mission baseline](validation/civilian-tribesman-chance-mission-baseline.txt),
[server final](validation/civilian-tribesman-chance-final.txt),
[mission final](validation/civilian-tribesman-chance-mission-final.txt), and
[static audit](validation/civilian-tribesman-chance-audit.json). Both validators
pass and match baseline. The follow-up changes only 40 `Chance` values to
0.75. Factions, one-AI sizes, disabled scavenging, equipment and caps remain
unchanged. Police source research is recorded in the implementation plan.

Size-reduction records are
[server baseline](validation/civilian-tribesman-singletons-baseline.txt),
[mission baseline](validation/civilian-tribesman-singletons-mission-baseline.txt),
[server final](validation/civilian-tribesman-singletons-final.txt),
[mission final](validation/civilian-tribesman-singletons-mission-final.txt), and
[static audit](validation/civilian-tribesman-singletons-audit.json). Both validators
pass with output matching baseline. Forty fixed single-AI definitions retain
`LootingBehaviour: NONE` and `CanBeLooted: 1`; role caps remain 20/20 and the
global cap is 61. Other settings and equipment are unchanged. Runtime checks
must verify one AI per patrol, no scavenging, and the 61-AI population ceiling.

Scavenging-phase records are
[server baseline](validation/civilian-tribesman-no-looting-baseline.txt),
[mission baseline](validation/civilian-tribesman-no-looting-mission-baseline.txt),
[server final](validation/civilian-tribesman-no-looting-final.txt),
[mission final](validation/civilian-tribesman-no-looting-mission-final.txt), and
[static audit](validation/civilian-tribesman-no-looting-audit.json). Both validators
pass with output matching baseline. Exactly 40 Civilian/Tribesman
`LootingBehaviour` fields change to `NONE`; all other settings remain unchanged.
After deploying the mission patrol file with staging stopped and restarting,
verify they ignore town loot while their corpses remain lootable. No server
launch or deployment was performed.

Earlier Civilian/Tribesman pair-update records are
[server baseline](validation/civilian-tribesman-pairs-baseline.txt),
[mission baseline](validation/civilian-tribesman-pairs-mission-baseline.txt),
[server final](validation/civilian-tribesman-pairs-final.txt),
[mission final](validation/civilian-tribesman-pairs-mission-final.txt), and
[static audit](validation/civilian-tribesman-pairs-audit.json). Both final
validators pass and match baseline. Checks confirm 40 fixed-size pairs,
unchanged 20/20 role patrol caps, global cap 101, the 101-living-AI ceiling,
and preservation of all other settings, including the new assassin anchor.
That earlier phase changed the assassin's waypoint, 40 patrol sizes and
one global cap. Runtime pair placement and performance remain unverified.

Spawn-relocation phase records are
[server baseline](validation/assasin-remote-spawn-baseline.txt),
[mission baseline](validation/assasin-remote-spawn-mission-baseline.txt),
[server final](validation/assasin-remote-spawn-final.txt),
[mission final](validation/assasin-remote-spawn-mission-final.txt), and
[static audit](validation/assasin-remote-spawn-audit.json). Both final validators
pass and match baseline. Static checks confirm exact official coordinates,
rarity and separation selection, 143 patrols / 45 loadouts, and that only the
assasin's `Waypoints` changed. All other patrol/profile/loadout values remain
unchanged. Placement and behavior require later staging verification.

Player relationship records are
[server baseline](validation/player-factions-baseline.txt),
[mission baseline](validation/player-factions-mission-baseline.txt),
[server final](validation/player-factions-final.txt),
[mission final](validation/player-factions-mission-final.txt), and
[static audit](validation/player-factions-audit.json). These records cover the
20 Tribesman faction changes, exact current mapping, 143 patrols / 45 loadouts,
and preservation of other patrol fields, profile settings, population caps and
all equipment. Both final `dzcli` validators pass and match their baselines;
the audit confirms only the 20 intended `Faction` values changed. These are
configuration checks; runtime relationships remain unverified.

Historical assasin configuration records are
[server baseline](validation/assasin-baseline.txt),
[mission baseline](validation/assasin-mission-baseline.txt),
[server final](validation/assasin-final.txt),
[mission final](validation/assasin-mission-final.txt), and
[static audit](validation/assasin-audit.json). Both final `dzcli` validators
pass, including mission validation of all 143 patrol definitions. The audit
covers 143 patrols / 45 loadouts, the new single-AI category and 61-AI ceiling,
exact 460-source correspondence, fixed equipment, and preservation of all
existing patrols/loadouts. Runtime acceptance remains a separate phase.

The earlier faction-alliance validation records are
[faction-alliances-baseline.txt](validation/faction-alliances-baseline.txt),
[faction-alliances-final.txt](validation/faction-alliances-final.txt), and
[faction-alliances-mission-final.txt](validation/faction-alliances-mission-final.txt).
Checks cover the 122 faction changes, unchanged Civilian entries, role counts,
and preservation of other patrol fields and loadouts. Both final validators
pass; server-root output matches the baseline. These are configuration checks,
not in-game verification.

Historical object-patrol validation is recorded in
[merc-object-baseline.txt](validation/merc-object-baseline.txt),
[merc-object-final.txt](validation/merc-object-final.txt), and
[merc-object-audit.json](validation/merc-object-audit.json). Checks cover JSON
parsing, 142 patrol definitions, 44 loadout files, all 51 classes, selector
references, profile order/probabilities, category caps, Merc settings, and
preservation of the original entries/equipment, AI settings, launchers, and
production during that phase. Profile/server and mission-root validators are both required.
No tests or custom scripts are added; no server launch or deployment is performed.

Later staging checks must cover ground placement, paired spawning and all kit
possibilities, inventory/capacity, the 61-AI ceiling, activation/despawn ranges
and timers, detection, prone behavior, scavenging, unlimited reload, movement,
player friendship with Civilians/Tribesmen, player/Merc hostility, cooperation
across Merc patrols, Civilian/Tribesman retaliation and friendly assistance,
assasin hostility/roaming/restart persistence, and host performance. When
Warlord patrols are added, verify their cooperation and hostility toward
players, Mercs, Civilians and Tribesmen.
These configuration checks are not in-game verification. See the
[assasin acceptance checklist](AI_IMPLEMENTATION.md#configuration-records-and-later-acceptance)
and the
[current acceptance checklist](AI_IMPLEMENTATION.md#configuration-checks-and-later-staging-acceptance).

The following historical baseline/final reports cover the initial ten-loadout
patrol phase. The later expansion uses separate `loadout-expansion-baseline.txt`,
`loadout-expansion-final.txt`, and `loadout-expansion-static-audit.json` reports
under `validation/`. Baseline and final profile/mission validation and the
independent static audit pass: 32 additions, 42 total loadouts, and 180 distinct
explicit inventory classes across the new files, all allowlisted. The baseline
file is a labeled summary of observed pre-edit results; the final report records
command output. Original JSON files are preserved. No tests or in-game checks
were performed for the loadout expansion.

Those six AI reports retain their original production paths as historical
evidence. The relocation preserves all 44 AI JSON file SHA-256 hashes and
restores the production server directory exactly to main `5c6306f`. Fresh
staging validator output is recorded in
[staging-ai-relocation.txt](validation/staging-ai-relocation.txt), with counts
and preservation checks in
[staging-ai-relocation-audit.json](validation/staging-ai-relocation-audit.json).
The launcher and inherited server settings were checked separately in
[staging-launcher-audit.json](validation/staging-launcher-audit.json): all five
mods have matching download/sync/load entries, the requested ports are set,
and existing paths, signing-key handling, and unrelated settings are preserved.

Repo-local `dzcli` v0.0.5 baseline validation reported no AI files, as expected;
the exact failure is preserved in
[validation/baseline-expansion-ai.txt](validation/baseline-expansion-ai.txt).
Final validation passes for the profiles and the mission patrol file:

```powershell
$dz = Join-Path (Get-Location) '.tools\dzcli\dzcli.exe'
& $dz validate expansion ai servers/first-light-badlands-nasdara-staging-us-east
& $dz validate expansion ai servers/first-light-badlands-nasdara-staging-us-east/mpmissions/dayzOffline.nasdara
```

Both commands are required with v0.0.5: server-root discovery does not reach the
nested mission settings. Supported mutations used dry-run previews; text mode
was required to apply them because this version's JSON mutation output does
not write files. AI settings and patrol-root settings used direct edits where
mutation commands are unavailable.

See [final validator output](validation/final-expansion-ai.txt) and the
[static audit](validation/expansion-ai-static-audit.json). All 12 configuration
JSON files in the original phase parse; that historical audit confirms 10 loadouts, 40 single-AI patrols, 45
allowlisted inventory classes, exact copied coordinates, cycling, caps,
quantities/chances, behavior and looting flags. No tests were added.
In-game placement, capacity, firearm loading, unlimited reload, scavenging,
movement, detection and relationships remain unverified.

## Windows host setup

Use Windows Command Prompt, SteamCMD, Robocopy, and a stable DayZ dedicated server
build supporting Nasdara. The scripts originate from the two batch launchers in
`../first-light-deer-isle-vanilla-us-east1/`. The manager is unchanged; the service
script uses the five mods listed above and staging game port `2901`.

| Script setting | Value |
| --- | --- |
| `STEAMCMD` | `C:\SteamCMD\steamcmd.exe` |
| `INSTALL` | `C:\DayZ\server` |
| `WSCONTENT` | `C:\SteamCMD\steamapps\workshop\content\221100` |
| Steam login | Same account as the reference, configured in both `+login` lines |
| Manager restart delay | 10 seconds |

Paths and Steam login are configured directly in `dayz-service.bat`, like the
reference. Keep all path variables and `-BEpath` consistent if adjusting them on
the host. The staging launcher deliberately retains `C:\DayZ\server`, including
profiles, keys, config, and BattlEye paths. Deploy it to the intended staging
host/installation; it does not provide isolation for a second process sharing
that directory. SteamCMD, Workshop paths, and `instanceId = 1` are unchanged.
The scripts take no command-line parameters.

1. Provision the Windows DayZ installation and a complete stock Nasdara mission
   under `C:\DayZ\server\mpmissions\dayzOffline.nasdara\`, using the server
   distribution or the complete official upstream mission linked above. Deploy
   `serverDZ.cfg` to `C:\DayZ\server\serverDZ.cfg`, then copy this directory's
   `mpmissions\dayzOffline.nasdara\db\messages.xml` over the installed mission's
   `db\messages.xml`, retaining all other installed mission files. Before the
   first modded start, copy
   `profiles/VPPAdminTools/` to `C:\DayZ\server\profiles\VPPAdminTools\`,
   `profiles/ExpansionMod/` to `C:\DayZ\server\profiles\ExpansionMod\`, and
   `mpmissions/dayzOffline.nasdara/expansion/settings/AIPatrolSettings.json` to
   `C:\DayZ\server\mpmissions\dayzOffline.nasdara\expansion\settings\AIPatrolSettings.json`.
   Create missing overlay directories. Retain
   existing deployed VPP settings and credentials on subsequent deployments,
   and review existing AI settings before replacing them. Keep a single
   `profiles` directory level; deploy the overlay without mirroring/deleting
   the stock mission or unrelated profile files.
   As with the reference, SteamCMD updates use `validate`; reapply the custom
   messages if Steam replaces that file during updates. The repository mission
   folder is a partial overlay; use file copies that preserve the stock mission.

2. Allow the staging network ports on the host and any relevant router/firewall.
   No RCon configuration is added.

3. From this staging server's repository directory in Command Prompt, run one
   update/start cycle:

   ```bat
   dayz-service.bat
   ```

   Or supervise repeated cycles:

   ```bat
   dayz-service-manager.bat
   ```

   The manager retries after every exit, with a 10-second delay. Each
   cycle updates DayZ and all five mods, synchronizes mod folders and signing keys,
   and runs `DayZServer_x64.exe` from the installation directory. Press Ctrl+C
   to stop the console manager; run only one manager per installation.

Logs and profiles use `C:\DayZ\server\profiles`. This is a console supervisor,
not a registered Windows service or timed restart scheduler.
Configuration/profile deployment is manual; the launcher mirrors mod directories
and never copies over admin profiles, AI settings, loadouts, or mission overrides.
No host deployment or live network verification was performed for this change.

After startup, verify browser visibility and join with `sillypassword`; check
that an empty or incorrect password is rejected. Check the live port bindings,
mission messages, and existing VPP access.
With matching client mods loaded, check the original 40 patrols near their spawn
areas and the new Merc pairs around eligible objects. Follow both the original
and [Merc acceptance checklist](AI_IMPLEMENTATION.md#configuration-checks-and-later-staging-acceptance).
All 16 Merc kits are referenced through selectors; Warlord and added Tribesman
.380 kits remain available for later assignments. The object definitions add
stock East and explicit Merc combat overrides without changing the launcher.

See [DayZ launch parameters](https://community.bistudio.com/wiki/DayZ:Server_Configuration).

## Staging baseline validation

The staging baseline was copied exactly from production at main `5c6306f`
before changing its hostname, game/client/query ports, and join password.
Repo-local `dzcli` v0.0.5 server and XML validation pass for both the source
and staging copy. Launchers were inspected without executing SteamCMD or DayZ.
VPP profiles, mission messages, historical validation reports, and unrelated
configuration remain unchanged. Live ports, password enforcement, VPP access,
and client connections still require deployment checks.

The sections below and `validation/baseline-*.txt` are historical production
records copied for provenance; their original names, paths, and results are
preserved and do not describe a staging runtime test.

## Initial mission validation

These historical checks cover the initial Linux setup. Validated with repo-local
`dzcli` v0.0.5. Baseline checks ran against the pristine
upstream import before customization, and final checks ran after configuration
and announcement changes.

| Check | Result |
| --- | --- |
| Exact upstream import | All 51 paths, modes, and blobs match the upstream Git tree |
| Final mission changes | Only `db/messages.xml` differs from the pristine import |
| Server configuration | Pass; hostname and mission set correctly |
| Recursive XML | Pass for all 44 XML files |
| Weather | Pass |
| Initialization script | Pass; static validation only |
| Bash launcher | `bash -n` passes; launcher was not executed |
| JSON syntax | All five imported JSON files parse |
| Inherited settings | All Frostline server fields match except hostname, mission, and removed VPP field |
| Frostline preservation | Unchanged Git tree versus branch baseline and unchanged working-file SHA-256 hashes |
| Economy and gameplay | Failures inherited from pristine upstream; final output identical to baseline |

Baseline diagnostics are preserved in
[validation/baseline-economy.txt](validation/baseline-economy.txt) and
[validation/baseline-gameplay.txt](validation/baseline-gameplay.txt).
Validation-log directory prefixes were updated with the Nasdara directory rename;
diagnostic content is unchanged.

Economy validation rejects empty event-spawn entries and Nasdara's
`OilPitArea`/`OilPitTrigger` values. It also warns about the upstream cattle
territory containing no live spawn zones. Gameplay validation rejects the
upstream lighting value and fields for construction, vehicles, bunker
broadcasts, external locks, and sandstorms. These failures were already present
before customization. These historical checks describe the original full import;
the host's complete stock mission now supplies the uncustomized files.

Frostline's original server config also fails v0.0.5 validation on the unannotated
`vppDisablePassword` field. The initial vanilla config omits that field and passes; it remains unchanged.
A successful live Nasdara startup and actual listener ports remain unverified.

## Base mod change validation

These historical checks cover the base mod change before the Windows conversion.
The base mod branch starts from merged `main` commit `38c17cf`. Repo-local
`dzcli` was checked for updates and remains at v0.0.5.

- `dzcli validate server` passes before and after the mod/profile additions;
  `serverDZ.cfg` is unchanged, including hostname and Nasdara mission selection.
- `bash -n dayz.sh` passes. The launcher was not executed.
- The three VPP profile files match Frostline byte for byte.
- The mission tree and complete Frostline directory remain identical to this
  branch's `main` baseline. Existing economy/gameplay diagnostics above remain
  applicable; mission files were not modified for these mods.
- Game port remains `2301`; query and RCon ports remain unspecified and unverified.

Mod downloads, server startup, client connections, VPP login, and live network
checks were not performed for that change.

## Windows batch launcher validation

These historical checks cover the batch conversion before stock-file cleanup.
The batch conversion branch starts from merged `main` commit `c44448c` (PR #55).

- Repo-local `dzcli` was installed/checked for updates at v0.0.5.
- `dzcli validate server` passes before and after the launcher replacement.
- `dayz-service-manager.bat` matches the Deer Isle reference byte for byte.
- `dayz-service.bat` differs from the reference only in the VPP mod entries and
  port `2301`. Both PowerShell launchers have been removed.
- The batch scripts were inspected statically and were not executed.
- The launcher selects `DayZServer_x64.exe`, port `2301`, CF then VPP, the
  existing logging flags, `battleye`, and `profiles` from the install directory.
- `serverDZ.cfg`, all mission/economy files, and VPP profiles remain unchanged
  from `main`; hostname, Nasdara mission, and VPP password checks are preserved.
- Frostline, Deer Isle, and all other server directories remain unchanged.

SteamCMD downloads, synchronization, process exit/restart behavior, VPP login,
and live network listeners still require verification on the Windows host.

## Custom override cleanup validation

The cleanup branch starts from merged `main` commit `585e16a` (PR #56).
Repo-local `dzcli` was checked for updates at v0.0.5.

- All 50 removed files match the pinned upstream Git blobs and modes exactly.
  `db/messages.xml` is the only customized mission file and remains unchanged.
- Server configuration and the remaining XML pass validation before and after
  cleanup. Both batch launchers, server settings, VPP profiles, and the authored
  historical validation logs remain unchanged. Other servers are unchanged.
- A standalone economy check on the partial repository mission reports the
  intentionally absent `cfgeconomycore.xml`. Validate a complete deployed mission.
- For comparison, a temporary full mission was assembled from the pinned stock
  files plus the retained messages override. XML, weather, and initialization
  validation pass. Economy/gameplay retain the inherited failures described
  above. All six validator outputs match the pre-cleanup baseline after
  normalizing mission paths.

The temporary validation mission and new logs are ignored under `.tools/`.
No installed server files, persistence, or live services were changed.
