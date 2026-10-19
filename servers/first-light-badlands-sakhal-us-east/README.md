# First Light Badlands Nasdara

Vanilla Nasdara configuration in `first-light-badlands-sakhal-us-east`.
The directory name retains the requested `sakhal` suffix; the mission runs Nasdara.
`../first-light-frostline-sakhal-us-east/` is the read-only configuration reference.

## Server settings and ports

| Setting | Value |
| --- | --- |
| Public hostname | `First Light | Badlands Nasdara | us-east1` |
| Mission template | `dayzOffline.nasdara` |
| Game port | `2301` (`-port=2301`, matching Frostline) |
| Steam query port | Unspecified, matching Frostline; runtime/default value unverified |
| RCon port | Unspecified, matching Frostline; runtime/default value unverified |
| Maximum players | 60 |
| Player view | First person; crosshair disabled |
| Time | System time, 2x day acceleration, 5x night multiplier, persistent |
| Mods | None |

The remaining server settings, Linux paths, logging, BattlEye path, service user,
restart policy, and shutdown signals mirror Frostline. VPP's
`vppDisablePassword` field is omitted. No Workshop downloads, mod launch arguments,
mod/key symlinks, or mod profiles are configured.

Only the configured game port is confirmed from repository files. Query/RCon
listeners and existing router/firewall forwarding require checking on the host.

## Mission provenance

All 51 files under `mpmissions/dayzOffline.nasdara/` were imported from
[BohemiaInteractive/DayZ-Central-Economy](https://github.com/BohemiaInteractive/DayZ-Central-Economy/tree/f220b2918b1d79a5055b1ee9be70c0ec1516e876/dayzOffline.nasdara).

- Upstream commit: `f220b2918b1d79a5055b1ee9be70c0ec1516e876`.
- Pristine import commit: `d206bc5`.
- Pristine mission Git tree: `1644633799219b52320e9220ead3fa1c8e1e90d3`.

The import commit preserves every upstream path, file mode, and byte, including
line endings. All 51 files remain present. The only subsequent mission change
is `db/messages.xml`, copied from Frostline to retain the server-name greeting,
Discord invite, and restart announcement. The other 50 files remain identical
to upstream, including the Nasdara loot economy, environment, weather, map data,
gameplay JSON, and initialization script.

The restart announcement is informational. This configuration does not install
a timed restart scheduler.

## Host setup

Use a Linux host with the `dayz` user/group, sudo access, SteamCMD at
`/usr/games/steamcmd`, and a stable DayZ dedicated server build supporting Nasdara.
The install location remains `/opt/dayz`, and the service remains `dayz.service`.

1. Run `sudo bash dayz.sh STEAM_USERNAME` with the intended Steam account. The
   script installs/updates DayZ, restores ownership to `dayz:dayz`, writes the
   systemd unit, and reloads systemd.
2. Deploy this directory's `serverDZ.cfg` to `/opt/dayz/serverDZ.cfg` and the
   complete `mpmissions/dayzOffline.nasdara/` tree to the corresponding mission
   location under `/opt/dayz/mpmissions/`.
3. Enable/start the service through the host's existing deployment procedure.

The launcher does not create the service account, copy repository configuration,
enable/start the service, or migrate Frostline persistence. Server logs and
profiles use `/opt/dayz/profiles`; no previous mod profiles are supplied here.
No host deployment or live network verification was performed for this change.

## Validation

Validated with repo-local `dzcli` v0.0.5. Baseline checks ran against the pristine
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

Exact baseline outputs are preserved in
[validation/baseline-economy.txt](validation/baseline-economy.txt) and
[validation/baseline-gameplay.txt](validation/baseline-gameplay.txt).

Economy validation rejects empty event-spawn entries and Nasdara's
`OilPitArea`/`OilPitTrigger` values. It also warns about the upstream cattle
territory containing no live spawn zones. Gameplay validation rejects the
upstream lighting value and fields for construction, vehicles, bunker
broadcasts, external locks, and sandstorms. These failures were already present
before customization; the official files are preserved for engine compatibility.

Frostline's original server config also fails v0.0.5 validation on the unannotated
`vppDisablePassword` field. The new vanilla server omits that field and passes.
A successful live Nasdara startup and actual listener ports remain unverified.
