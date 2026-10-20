# First Light Badlands Nasdara Staging

Password-protected Nasdara staging configuration with CF, VPPAdminTools, and Expansion AI in
`first-light-badlands-nasdara-staging-us-east`, copied from
`../first-light-badlands-nasdara-us-east/` at main commit `5c6306f`.
`../first-light-frostline-sakhal-us-east/` is the read-only configuration reference.
The mission directory contains only custom overrides for a complete installed
stock Nasdara mission.

PR #58 adds the complete AI overlay to this staging server. Its 42 loadouts
and 40 patrol definitions are unchanged by relocation; the original production
server remains at its main baseline. Deployment and runtime testing are pending.

## Server settings and ports

| Setting | Value |
| --- | --- |
| Public hostname | `First Light | Badlands Nasdara | STAGING | us-east1` |
| Join password | `sillypassword` (intentionally public; discourages casual joins) |
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
unchanged. The join password does not change DayZ admin or VPP credentials.
Deer Isle vanilla also uses game port 2901 and query port 27018; these servers
need separate host/IP bindings when run concurrently.

## Mission override and provenance

The mission overlay includes `mpmissions/dayzOffline.nasdara/db/messages.xml` and
`mpmissions/dayzOffline.nasdara/expansion/settings/AIPatrolSettings.json`.
The messages retain Frostline's server-name greeting and Discord invite, and add
the Deer Isle Expansion shutdown countdown. Provision the full stock mission on the host, then copy messages
into its existing `db` directory and the patrol settings into its
`expansion/settings` directory, preserving all stock mission files.

The original 51-file mission import came from
[BohemiaInteractive/DayZ-Central-Economy](https://github.com/BohemiaInteractive/DayZ-Central-Economy/tree/f220b2918b1d79a5055b1ee9be70c0ec1516e876/dayzOffline.nasdara).

- Upstream commit: `f220b2918b1d79a5055b1ee9be70c0ec1516e876`.
- Pristine import commit: `d206bc5`.
- Pristine mission Git tree: `1644633799219b52320e9220ead3fa1c8e1e90d3`.

The pristine import preserved every upstream path, file mode, and byte. A
comparison of Git blobs and modes confirmed that the other 50 files were still
unchanged stock files. They have been removed from this directory, including
`areaflags.map`, the economy databases, environment territories, map placement
data, stock XML/JSON settings, weather, and `init.c`. The installed stock mission
supplies these files. The original complete import remains in Git history.

The mission's `db/messages.xml` uses DayZ's built-in graceful shutdown countdown,
matching Deer Isle Expansion: `<deadline>180</deadline>` and `<shutdown>1</shutdown>`.
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

The overlay provides **42 loadout definitions**: four Civilian, nine Tribesman,
13 Warlord, and 16 Western_Merc, including one MP5 medic. The original 40
single-AI patrol definitions still reference only the original four Civilian
and six Tribesman loadouts at the 20 official player spawn-area centers. The
32 new loadout definitions are prepared for later assignment; Warlord/Merc
factions, patrols, accuracy overrides, and deployment remain pending. Full
equipment tables, probabilities, sources, and acceptance limits are recorded in
[AI_IMPLEMENTATION.md](AI_IMPLEMENTATION.md#loadout-expansion-phase-2026-10-20).

| Repository path | Deployment destination |
| --- | --- |
| `profiles/ExpansionMod/Loadouts/Nasdara_Civilian_01.json` through `04.json` | `<active-profile>/ExpansionMod/Loadouts/`, same filenames |
| `profiles/ExpansionMod/Loadouts/Nasdara_Tribesman_01.json` through `09.json` | `<active-profile>/ExpansionMod/Loadouts/`, same filenames |
| `profiles/ExpansionMod/Loadouts/Nasdara_Warlord_01.json` through `13.json` | `<active-profile>/ExpansionMod/Loadouts/`, same filenames |
| `profiles/ExpansionMod/Loadouts/Nasdara_Western_Merc_01.json` through `16.json` | `<active-profile>/ExpansionMod/Loadouts/`, same filenames; 11 is `Nasdara_Western_Merc_11_Medic.json` |
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
| Civilian | Badlands Kameez/short-sleeve tops, Shalwar pants, regional hats/sandals/vests; one knife, hatchet or pipe | Firearms, melee, bandages and clothing upgrades |
| Tribesman | Badlands Kameez/Shalwar or olive OBR outfit, regional hats/sandals and chest rigs; original shotgun/Mosin/SKS definitions plus three .380 SMG additions | Existing patrols retain their original six loadouts and looting policy |
| Warlord | Traditional regional clothing; probabilistic tactical/ballistic vest; Soviet weapons and regional additions, full magazines or limited loose ammunition | New loadout definitions only; patrol behavior deferred |
| Western_Merc | Matching USMC desert uniform, plate carrier/pouches, helmet, backpack; western firearms, optics and compatible suppressors, medical supplies | New loadout definitions only; patrol behavior deferred |

All starting items are pristine. Explicit inventory class names use
the pinned official Nasdara
[types.xml](https://github.com/BohemiaInteractive/DayZ-Central-Economy/blob/f220b2918b1d79a5055b1ee9be70c0ec1516e876/dayzOffline.nasdara/db/types.xml).
The original ten loadouts choose headwear, footwear and vests uniformly;
Tribesman_06 has a fixed OBR cap. Some loadouts independently roll bottles,
bandages or matches; three have no helper supplies. Civilians start without
firearms/ammunition but can acquire firearms while scavenging. Clothing and
corpse contents can change after spawning.

The original six Tribesman loadouts start with one small compatible loose-ammo stack: normalized quantity
0.20 for shotgun shells, 0.25 for Mosin cartridges or 0.40 for SKS cartridges
(2 buckshot shells in a ten-shell stack, or 5/8 rifle rounds in 20-round stacks). `UnlimitedReload: 1` replenishes
ammunition during use and can leave more ammo on a corpse than initially given.
Bottles start at 35-65% capacity with native liquid defaults.

The existing Civilian/Tribesman patrols use stock `Civilian` faction behavior and `ROAMING_LOCAL`, walk
outside combat, and sprint under threat. Threat, noise investigation and
flanking distances are 45.72 m (50 yards), with flanking outside combat disabled.
Activation distances are 45.72-500 m; despawn distance is 600 m, despawn time
300 seconds and respawn time 1800 seconds. Patrols are lootable and do not
persist across restarts. Caps are 40 globally, 20 Civilian and 20 Tribesman
patrols for every player count from 0 through 255.

`PlayerFactions: ["West"]` provides friendship with these Civilian-faction
patrols. Friendly/guard recruitment is disabled. Stock retaliation and friendly
assistance remain enabled; existing persisted player-group factions may need
separate attention during deployment. Warlord and Western_Merc faction/patrol
implementation remains deferred, while their loadout definitions are included.

### Additional loadout inventory

The original ten loadout files are preserved. Tribesman_07/08/09 copy the approved
01/02/03 clothing and supplies with CZ61, RAK, or Bizon respectively, one full
attached magazine, no spares, and no loose ammunition. The Bizon includes its
buttstock. Existing patrol assignments are unchanged.

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

For a later deployment, install compatible versions on the server and connecting
clients, load dependencies before Expansion AI, and deploy their signing keys
following the official guide. Keep VPP's existing CF dependency. Copy this
server's `profiles/ExpansionMod` overlay to the active profile directory and
the patrol file to the installed stock Nasdara mission. Preserve installed
mission data and unrelated existing profile settings; review any existing AI
settings before replacing them. Stock roaming-location settings generated for
Nasdara still require runtime inspection.

The staging launcher now installs/updates, synchronizes, and loads these AI
dependencies alongside CF/VPP. It copies their signing keys using the existing
helper. No custom mod is added. The launcher was inspected without running
SteamCMD or DayZ; actual installation, startup, and runtime acceptance remain
deployment work. The `codex/nasdara-staging` baseline branch retains CF/VPP only;
deploy `codex/nasdara-ai-plan` for this AI-enabled version.

### Configuration validation

The following original baseline/final reports cover the initial ten-loadout
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
With matching client mods loaded, check that the 40 configured patrols activate
near their spawn areas and verify the runtime checklist in
[AI_IMPLEMENTATION.md](AI_IMPLEMENTATION.md#configuration-validation-and-later-acceptance).
Only the original ten loadouts are assigned; the 32 additional definitions
remain available for later assignments. No new factions or combat-tier
overrides are introduced by enabling the launcher.

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
