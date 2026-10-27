[![dzdeploy](https://github.com/jsirianni/dayz_config/actions/workflows/dzdeploy.yml/badge.svg)](https://github.com/jsirianni/dayz_config/actions/workflows/dzdeploy.yml)

# DayZ Config

DayZ configuration and the Go `dzdeploy` CLI for Windows and Linux clients managing the three Windows servers under `servers/current/`.

## Contents

- [Deployment](#deployment)
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

## Server status

Last verified **October 26, 2026 at 10:55 p.m. EDT (America/New_York)** using SSH and fresh DZSA checks for Nasdara and Deer Isle. These are recorded observations; player counts and running state can change. Nasdara AI has no configured endpoint, so its live state is unknown.

The managed servers use DayZ's native 180-minute shutdown countdown in `messages.xml`. Their Windows services remain running and supervise updates, configuration reapplication, and game relaunch after exit. A complete three-hour shutdown/relaunch cycle has **not yet been observed** on either managed server.

### Nasdara (standard)

Deployment alias: `nasdara`. Source configuration: [first-light-badlands-nasdara-us-east](servers/current/first-light-badlands-nasdara-us-east).

The server at **192.168.1.196** was running with **0/60 players**. DayZ is installed at `C:\DayZ\server` and managed by the running, automatic-start Windows service `dzdeploy-nasdara`. The legacy `DayZServer` service is stopped and disabled.

- **UDP ports:** game `2301`, BattlEye `2303`, Steam query `27016`.
- **Public discovery:** [DZSA server check](https://dayzsalauncher.com/#/servercheck/50.36.224.4:27016) passed and verified both expected mods: CF and VPPAdminTools.
- **Three-hour shutdown/relaunch:** configured; supervision active.
- **Installed revision:** `1ab8d0629ad4631dff691c052a4e72e0dc7815ba`.
- **Last managed startup revision:** `1ab8d0629ad4631dff691c052a4e72e0dc7815ba`.
- **Restart pending:** no.

The agent reported no active deployment error. It remains on the revision preceding PR #85's fix for Steam hostname truncation; its existing shorter hostname passes readiness checks.

### Deer Isle

Deployment alias: `deer-isle`. Source configuration: [first-light-deer-isle-expansion-us-east1](servers/current/first-light-deer-isle-expansion-us-east1).

The server at **192.168.1.190** was running with **0/60 players**. DayZ is installed at `C:\DayZ\server` and managed by the running, automatic-start Windows service `dzdeploy-deer-isle`. The legacy `DayZServer` service is stopped and disabled. The existing saved world was preserved and backed up during migration.

- **UDP ports:** game `4801`, BattlEye `4803`, Steam query `27019`.
- **Public discovery:** [DZSA server check](https://dayzsalauncher.com/#/servercheck/50.36.224.4:27019) passed and verified all 13 expected mods.
- **Three-hour shutdown/relaunch:** configured; supervision active.
- **Installed revision:** `a970ecad7cfc0a8cac6774a772f3d9f2ae87e0a4`.
- **Last managed startup revision:** `a970ecad7cfc0a8cac6774a772f3d9f2ae87e0a4`.
- **Restart pending:** no.

The upgraded agent includes PR #85's hostname-length fix, and managed startup completed successfully. Startup logs still contain mod exceptions, including NULL-pointer errors and missing ruined-wheel classes for `jmc_atv`, `Runner`, `DodgeRam_3500`, and `Honda_Civic`. These were present before and after the agent upgrade; the game remained running and passed query checks. Their gameplay impact remains unverified.

### Nasdara AI

Deployment alias: `nasdara-staging`. Source configuration: [first-light-badlands-nasdara-staging-us-east](servers/current/first-light-badlands-nasdara-staging-us-east).

This server has been left untouched during the Nasdara and Deer Isle rollouts. Its IP address is not configured in the deployment client's ignored YAML, and it has not been enrolled here. Its game process, players, service, live ports, and DZSA discovery are **unverified**; this does not establish that the server is stopped.

- **Three-hour shutdown/relaunch:** the 180-minute native shutdown is configured in Git, but it has not been deployed or verified on this host by this rollout.
- **Installed revision:** none recorded by `dzdeploy`.
- **Last managed startup revision:** none recorded by `dzdeploy`.
- **Restart pending:** unknown.

The next step is to obtain its server IP, verify bootstrap/SSH access and existing ports, then enroll and deploy it when explicitly requested.

## Resources

- [Hosting a Linux Server](https://community.bistudio.com/wiki/DayZ:Hosting_a_Linux_Server)
- [Enforce Script Syntax](https://community.bistudio.com/wiki/DayZ:Enforce_Script_Syntax)
- [XML to Go Struct](https://xml-to-go.github.io/)
- [Understanding Event options](https://vocal.media/gamers/day-z-console-modding-part-2)
