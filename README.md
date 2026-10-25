[![dzdeploy](https://github.com/jsirianni/dayz_config/actions/workflows/dzdeploy.yml/badge.svg)](https://github.com/jsirianni/dayz_config/actions/workflows/dzdeploy.yml)

# Dayz Config

DayZ configuration and the Go `dzdeploy` CLI for Windows and Linux clients managing the three Windows servers under `servers/current/`.

Build: `go build -o .tools/bin/ ./cmd/dzdeploy`. Populate the ignored `deploy.yaml` using `deploy.example.yaml`, then follow [the deployment runbook](docs/deployment.md). Routine deployment uses native Go SSH/SFTP and a Windows service, with no PowerShell dependency.

Fresh Windows servers need two provisioning steps: run `bootstrap-windows.ps1` as Administrator with your public key and allowed source network, then run `dzdeploy deploy <target> --restart` on the client. Deployment installs prerequisites, handles Steam login prompts, and enrolls the Windows service automatically. Portable client packages are built with `go run ./cmd/dzdeploy-package` and published as CI artifacts.

```text
dzdeploy doctor nasdara-staging
dzdeploy deploy nasdara-staging --dry-run
dzdeploy deploy nasdara-staging
dzdeploy deploy nasdara-staging --restart
```

Deployment requires clean main matching freshly fetched origin/main. Restarts are explicit. Configuration is reapplied after Steam updates; persistence/runtime data are excluded. VPP uses SteamID admin allowlists with its password prompt disabled. Resources below include historical Linux hosting notes.

# Resources

- [Hosting a Linux Server](https://community.bistudio.com/wiki/DayZ:Hosting_a_Linux_Server)
- [Enforce Script Syntax](https://community.bistudio.com/wiki/DayZ:Enforce_Script_Syntax)
- [XML to Go Struct](https://xml-to-go.github.io/)
- [Understanding Event options](https://vocal.media/gamers/day-z-console-modding-part-2)
