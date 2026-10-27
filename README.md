[![dzdeploy](https://github.com/jsirianni/dayz_config/actions/workflows/dzdeploy.yml/badge.svg)](https://github.com/jsirianni/dayz_config/actions/workflows/dzdeploy.yml)

# DayZ Config

DayZ configuration and the Go `dzdeploy` CLI for Windows and Linux clients managing the three Windows servers under `servers/current/`.

## Contents

- [Deployment](#deployment)
- [DZSA notifier](#dzsa-notifier)
- [Servers](#servers)
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

## Servers

All three Windows servers are enrolled with `dzdeploy`, use `C:\DayZ\server`, and run under an automatic-start `dzdeploy-<alias>` service. Their legacy `DayZServer` services are disabled.

DayZ performs a native shutdown after **180 minutes of mission uptime**, using the countdown in `messages.xml`. The managed service then updates the server and mods, reapplies managed configuration, and launches DayZ again. Automatic shutdown and relaunch have been verified on all three servers. Update and loading time add to the interval between launches.

Use `dzdeploy status <alias>` for deployment state and `dzdeploy check <alias>` for fresh public discovery and mod verification. Installed revision, last managed startup revision, and restart-pending state are runtime information; consult the commands for current values.

### Nasdara (standard)

Deployment alias: `nasdara`. Source configuration: [first-light-badlands-nasdara-us-east](servers/current/first-light-badlands-nasdara-us-east).

- **LAN address:** `192.168.1.196`.
- **Service:** `dzdeploy-nasdara`.
- **UDP ports:** game `2301`, BattlEye `2303`, Steam query `27016`.
- **Public discovery:** [DZSA server check](https://dayzsalauncher.com/#/servercheck/50.36.224.4:27016).
- **Mods:** CF and VPPAdminTools.

### Deer Isle

Deployment alias: `deer-isle`. Source configuration: [first-light-deer-isle-expansion-us-east1](servers/current/first-light-deer-isle-expansion-us-east1).

- **LAN address:** `192.168.1.190`.
- **Service:** `dzdeploy-deer-isle`.
- **UDP ports:** game `4801`, BattlEye `4803`, Steam query `27019`.
- **Public discovery:** [DZSA server check](https://dayzsalauncher.com/#/servercheck/50.36.224.4:27019).
- **Mods:** 13 managed Workshop mods, including DeerIsle, DayZ Expansion, Terje, and vehicle mods.

Known startup mod errors include NULL pointers, duplicate `hatchback_exhaust` particles, and missing ruined-wheel classes for `jmc_atv`, `Runner`, `DodgeRam_3500`, and `Honda_Civic`. Scheduled restarts and discovery checks succeed despite these errors; their gameplay impact remains unverified.

### Nasdara AI

Deployment alias: `nasdara-staging`. Source configuration: [first-light-badlands-nasdara-staging-us-east](servers/current/first-light-badlands-nasdara-staging-us-east).

- **LAN address:** `192.168.1.224`.
- **Service:** `dzdeploy-nasdara-staging`.
- **UDP ports:** game `2901`, BattlEye `2903`, Steam query `27018`.
- **Public discovery:** [DZSA server check](https://dayzsalauncher.com/#/servercheck/50.36.224.4:27018).
- **Mods:** CF, VPPAdminTools, Dabs Framework, DayZ-Expansion-Core, and DayZ-Expansion-AI.

## Resources

- [Hosting a Linux Server](https://community.bistudio.com/wiki/DayZ:Hosting_a_Linux_Server)
- [Enforce Script Syntax](https://community.bistudio.com/wiki/DayZ:Enforce_Script_Syntax)
- [XML to Go Struct](https://xml-to-go.github.io/)
- [Understanding Event options](https://vocal.media/gamers/day-z-console-modding-part-2)
