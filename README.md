[![dzdeploy](https://github.com/jsirianni/dayz_config/actions/workflows/dzdeploy.yml/badge.svg)](https://github.com/jsirianni/dayz_config/actions/workflows/dzdeploy.yml)

# DayZ Config

DayZ configuration and the Go `dzdeploy` CLI for Windows and Linux clients managing the three Windows servers under `servers/current/`.

## Contents

- [Deployment](#deployment)
- [DZSA notifier](#dzsa-notifier)
- [Server status](#server-status)
  - [Nasdara (standard)](#nasdara-standard)
  - [Deer Isle](#deer-isle)
  - [Nasdara AI](#nasdara-ai)
- [Resources](#resources)

## Deployment

Build: `go build -o .tools/bin/ ./cmd/dzdeploy`. Populate the ignored `deploy.yaml` using `deploy.example.yaml`, then follow [the deployment runbook](docs/deployment.md). Routine deployment uses native Go SSH/SFTP and a Windows service, with no PowerShell dependency.

Fresh Windows servers need two provisioning steps: copy only [bootstrap-windows.ps1](scripts/bootstrap-windows.ps1) to the server and run it as Administrator with the allowed source network, then run `dzdeploy deploy <target> --restart` on the client. The script already embeds the deployment client's public key. Deployment installs prerequisites, handles Steam login prompts, trusts the SSH host on first connection, and enrolls the Windows service automatically. Portable client packages are built with `go run ./cmd/dzdeploy-package` and published as CI artifacts.

```text
dzdeploy doctor nasdara-staging
dzdeploy deploy nasdara-staging --dry-run
dzdeploy deploy nasdara-staging
dzdeploy deploy nasdara-staging --restart
```

Deployment requires clean main matching freshly fetched origin/main. Restarts are explicit. Configuration is reapplied after Steam updates; persistence/runtime data are excluded. VPP uses SteamID admin allowlists with its password prompt disabled. Resources below include historical Linux hosting notes.

## DZSA notifier

[dzsa-notifier](dzsa-notifier/README.md) is an independent Linux service that
periodically submits a configured list of public query endpoints to DZSA. It
provides Prometheus metrics, systemd DEB/RPM packages, nonroot amd64/arm64
distroless containers, and a PowerShell script for managing its configuration over
SSH. Its GoReleaser workflow reserves repository `v*` release tags for this app.
The notifier has not been deployed to a Linux host as part of this change.

## Server status

Verification snapshots, using SSH and fresh DZSA checks (America/New_York):

- **All three servers:** October 27, 2026 at 4:59 p.m. EDT. SSH verified game processes, services, listeners, agent heartbeats/state, startup results, and DayZ shutdown logs; fresh DZSA checks verified discovery and expected Workshop mod sets.

These are recorded observations; player counts and running state can change. All three servers are enrolled and managed by `dzdeploy`.

The managed servers use DayZ's native 180-minute shutdown countdown in `messages.xml`. Their Windows services remain running and supervise updates, configuration reapplication, and game relaunch after exit. Retained logs and successful managed startup results now confirm **six complete scheduled shutdown/relaunch cycles each for Nasdara and Deer Isle, and five for Nasdara AI** since their managed setup on October 26. Logs show the native warning countdown, player saving, mission finish, and successful termination before automatic relaunch at roughly three-hour intervals. Update/loading and discovery checks add time around each cycle.

### Nasdara (standard)

Deployment alias: `nasdara`. Source configuration: [first-light-badlands-nasdara-us-east](servers/current/first-light-badlands-nasdara-us-east).

The server at **192.168.1.196** was running with **0/60 players**. DayZ is installed at `C:\DayZ\server` and managed by the running, automatic-start Windows service `dzdeploy-nasdara`. The legacy `DayZServer` service is stopped and disabled.

- **UDP ports:** game `2301`, BattlEye `2303`, Steam query `27016`.
- **Public discovery:** [DZSA server check](https://dayzsalauncher.com/#/servercheck/50.36.224.4:27016) passed and verified both expected mods: CF and VPPAdminTools.
- **Three-hour shutdown/relaunch:** six completed cycles verified; supervision active. Latest clean shutdown at 4:20:28 p.m. EDT; current game process started at 4:20:56 p.m. and managed startup passed DZSA at 4:21:05 p.m.
- **Installed revision:** `1ab8d0629ad4631dff691c052a4e72e0dc7815ba`.
- **Last managed startup revision:** `1ab8d0629ad4631dff691c052a4e72e0dc7815ba`.
- **Restart pending:** no.

The agent heartbeat was fresh, supervision was unpaused, and no active deployment error or recovery journal was present. Historical setup failures predate these successful cycles. The agent remains on the revision preceding PR #85's fix for Steam hostname truncation; its existing shorter hostname passes readiness checks.

### Deer Isle

Deployment alias: `deer-isle`. Source configuration: [first-light-deer-isle-expansion-us-east1](servers/current/first-light-deer-isle-expansion-us-east1).

The server at **192.168.1.190** was running with **0/60 players**. DayZ is installed at `C:\DayZ\server` and managed by the running, automatic-start Windows service `dzdeploy-deer-isle`. The legacy `DayZServer` service is stopped and disabled. The existing saved world was preserved and backed up during migration.

- **UDP ports:** game `4801`, BattlEye `4803`, Steam query `27019`.
- **Public discovery:** [DZSA server check](https://dayzsalauncher.com/#/servercheck/50.36.224.4:27019) passed and verified all 13 expected mods.
- **Three-hour shutdown/relaunch:** six completed cycles verified; supervision active. Latest clean shutdown at 4:56:40 p.m. EDT; current game process started at 4:57:10 p.m. and managed startup passed DZSA at 4:57:24 p.m.
- **Installed revision:** `a970ecad7cfc0a8cac6774a772f3d9f2ae87e0a4`.
- **Last managed startup revision:** `a970ecad7cfc0a8cac6774a772f3d9f2ae87e0a4`.
- **Restart pending:** no.

The upgraded agent includes PR #85's hostname-length fix. Its heartbeat was fresh, supervision was unpaused, and no active deployment error or recovery journal was present. Current startup logs still contain mod exceptions, including NULL-pointer errors, a duplicate `hatchback_exhaust` particle warning, and missing ruined-wheel classes for `jmc_atv`, `Runner`, `DodgeRam_3500`, and `Honda_Civic`. The game completed its scheduled cycles and passed query checks despite these errors. Their gameplay impact remains unverified.

### Nasdara AI

Deployment alias: `nasdara-staging`. Source configuration: [first-light-badlands-nasdara-staging-us-east](servers/current/first-light-badlands-nasdara-staging-us-east).

The server at **192.168.1.224** was running with **1/60 players**. DayZ is installed at `C:\DayZ\server` and managed by the running, automatic-start Windows service `dzdeploy-nasdara-staging`. The legacy `DayZServer` service is stopped and disabled. The existing saved world was preserved and backed up during migration.

- **UDP ports:** game `2901`, BattlEye `2903`, Steam query `27018`.
- **Public discovery:** [DZSA server check](https://dayzsalauncher.com/#/servercheck/50.36.224.4:27018) passed and verified all five expected mods: CF, VPPAdminTools, Dabs Framework, DayZ-Expansion-Core, and DayZ-Expansion-AI.
- **Three-hour shutdown/relaunch:** five completed cycles verified; supervision active. Latest clean shutdown at 2:32:50 p.m. EDT; current game process started at 2:33:07 p.m. and managed startup passed DZSA at 2:33:24 p.m.
- **Installed revision:** `d6f26d3f02d3d5df30d1da187cb0b6ccd001a7a5`.
- **Last managed startup revision:** `d6f26d3f02d3d5df30d1da187cb0b6ccd001a7a5`.
- **Restart pending:** no.

The agent heartbeat was fresh, supervision was unpaused, and no active deployment error or recovery journal was present. Configuration and agent hashes matched deployment sources during the October 26 migration verification; those hashes were not rechecked during this restart audit.

## Resources

- [Hosting a Linux Server](https://community.bistudio.com/wiki/DayZ:Hosting_a_Linux_Server)
- [Enforce Script Syntax](https://community.bistudio.com/wiki/DayZ:Enforce_Script_Syntax)
- [XML to Go Struct](https://xml-to-go.github.io/)
- [Understanding Event options](https://vocal.media/gamers/day-z-console-modding-part-2)
