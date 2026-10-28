# Go DayZ deployment

## Build and configure

```text
go build -o .tools/bin/ ./cmd/dzdeploy
```

Windows produces `.tools/bin/dzdeploy.exe`; Linux produces `.tools/bin/dzdeploy`. These are the only supported client platforms. Add the directory to PATH or use its full path. The module selects Go 1.27.3. Go is not needed to run the compiled executable. Git is required on the client. Fresh Windows Server 2019/2022/2025 x64 hosts need no Go, SteamCMD, or DayZ installation.

For a portable distribution containing the client, Windows host companion, SSH script, example YAML, and checksums, run `go run ./cmd/dzdeploy-package`. `--os windows|linux --arch amd64|arm64` selects the client platform. CI publishes Windows/Linux ZIP artifacts. Keep `agents/windows-amd64/dzdeploy.exe` beside the extracted Linux client; Windows amd64 clients upload their own executable. Source builds can select another companion through YAML's optional `agent_binary` path.

`deploy.yaml` is the single user-maintained configuration file. A local placeholder copy is provided; fresh clones should copy `deploy.example.yaml` to `deploy.yaml`. Both that populated file and `.deploy/` are ignored by Git. Fill the three host addresses, SSH sources, paths, Steam account names, and verified listener ports. Unknown fields, duplicate keys, multiple documents, and unsupported aliases/versions are rejected.

| Alias | Source under servers/current | Game port | Query port |
| --- | --- | ---: | --- |
| `nasdara-staging` | `first-light-badlands-nasdara-staging-us-east` | 2901 | 27018 |
| `nasdara` | `first-light-badlands-nasdara-us-east` | 2301 | 27016 |
| `deer-isle` | `first-light-deer-isle-expansion-us-east1` | 4801 | 27019 |

Ports are part of the operator's router/firewall forwarding configuration. Preserve them during deployment work; change a port only when the operator explicitly requests that specific change. Nasdara's public UDP listeners are 2301, 2303, and 27016 (Steam query). Query ports are explicit in tracked configuration, and `query_address` must match. Different hosts can reuse the same ports; only instances sharing an IP need nonconflicting ports. Example RCon ports are configurable choices.

Nasdara installations previously enrolled with query port 27017 need the host's protected `host.json` query address and managed game firewall rule corrected to 27016 along with the client YAML and deployed `serverDZ.cfg`. Back up those settings, reload the host agent configuration, and perform an explicitly authorized game restart. Editing client YAML alone does not update an enrolled host or the running game's listener. Keep the router's existing port forwarding unchanged.

SSH key paths are relative to YAML. Omit `key_file` to use Windows OpenSSH's agent or Unix `SSH_AUTH_SOCK`. Encrypted key files can use `key_passphrase_env: VARIABLE_NAME`. Keep private keys and passwords out of YAML. Git's `serverDZ.cfg` is copied exactly, including its existing game password values.

The nested `host` settings drive automatic enrollment; no second user-maintained file is needed. Enrollment writes private machine-readable `host.json` for the service. Launcher mods/order/game ports remain tracked in `internal/deploy/targets.json` and are packaged from the deployment commit. `remote_dir` must be the SFTP spelling of `host.state_dir`, such as `/C:/ProgramData/DayZDeploy/nasdara-staging`.

For public DZSA startup verification, set the optional `host.public_query_address` to the existing public IPv4 address and forwarded UDP query port, for example `'203.0.113.10:27016'`. This is the address used by the [DZSA server checker](https://dayzsalauncher.com/#/servercheck), and can differ from `query_address` when NAT translates the query port. It never changes any listener or forwarding. Fresh enrollment saves it with the other host settings. Leaving it empty preserves local query startup checks.

## Two-step provisioning, staging first

Have the client SSH key and populated local `deploy.yaml` available. The host must have internet access for GitHub release downloads, Windows features, vendor installers, Steam, and the pinned stock mission source. Steam authentication requires an account entitled to the configured downloads. Keep the private SSH key on the client.

**Step 1 — on each Windows server**, copy only `scripts/bootstrap-windows.ps1`, which already contains the deployment client's public key. Run it in an elevated Windows PowerShell terminal:

```powershell
.\bootstrap-windows.ps1 -AllowedSource '203.0.113.10/32'
```

The same script works on all three hosts without edits or a separate key file. `-PublicKeyPath` remains available for older workflows; it is unnecessary for this single-file workflow. The matching private key stays on the deployment client.

`-UserName` defaults to the existing enabled local `Administrator`; `-SshPort` defaults to 22. `-AllowedSource` accepts explicit IPv4/IPv6 addresses or CIDRs, including multiple entries. Use the same username/port in client YAML. `-WhatIf` previews setup without changes. Built-in Windows PowerShell 5.1 is sufficient; no modules, Go executable, repository, or game installation are needed on the host.

The script reuses a complete existing OpenSSH installation, or tries Windows Client and Server capabilities. If Windows cannot install them, it automatically downloads and verifies the pinned Microsoft OpenSSH package. It preserves host keys and existing authorized keys, configures key-only SSH command execution and SFTP, and scopes a firewall rule to the supplied sources. It disables known broad OpenSSH rules, validates `sshd_config` using the selected service executable, and starts SSH automatically. On setup failure it restores the original configuration, keys, permissions, firewall rules, and service settings. Backups remain in protected `%ProgramData%\ssh\dzdeploy-backup-*` folders. No Windows Update policy changes or forced reboots are performed.

**Step 2 — on the client**, run:

```text
dzdeploy deploy nasdara-staging --restart
```

The first deploy automatically trusts the SSH host key received on connection and saves it in `known_hosts`. No fingerprint retrieval, keyscan, confirmation, or extra command is required. Subsequent commands reject changed host keys. An optional `--host-key SHA256:...` requires a specific fingerprint; it cannot override established trust. A dry-run accepts an unknown host for that connection without saving trust. First-use trust relies on reaching the intended server on the network during that initial connection.

If `host.steam_user` is empty or still `REPLACE_STEAM_ACCOUNT`, deploy asks for the Steam account name within the command and enrolls the service with it. Configured names are reused. A dry-run does not prompt. Steam password, Guard, and mobile approval prompts also stay inside deploy; no separate SteamCMD login is required.

The first deploy uploads and read-back verifies its Windows agent, installs SteamCMD and signed Microsoft Visual C++ prerequisites, creates protected directories, sets up loopback RCon/game firewall rules, and installs service `dzdeploy-<alias>`. The service uses `NT SERVICE\dzdeploy-<alias>`, a password-free Windows virtual account. SteamCMD authentication runs under that identity: any password, Steam Guard code, or mobile approval is requested inside the client command. Secrets travel through SSH and a restricted local named pipe; answers never enter command arguments, YAML, persistent results, or logs. Steam's cached login is protected in the SteamCMD directory and reused.

The service downloads DayZ/mods, supplies the complete mission, reapplies the repository overlays, and launches only when `--restart` was supplied. Missing Nasdara stock missions come from official Central Economy commit `f220b2918b1d79a5055b1ee9be70c0ec1516e876`; existing stock files are preserved. Deer Isle's complete tracked mission is deployed to its installed location. No manual SteamCMD login, account creation, host binary copy, mission copy, or extra bootstrap command is required.

Without `--restart`, initial provisioning and configuration copying complete with DayZ stopped and activation pending. A first `--dry-run` reports planned provisioning without uploading or installing anything. Initial Steam prompts complete before `--no-wait` can return a submitted operation. During later `deploy --restart`, authentication is checked before stopping the game.

Enrollment progress is recorded in protected `enrollment.json`; `status` can report it even before the service has created deployment state. If setup or its SSH connection fails, rerun the deploy command to resume. Existing healthy deployments are reused; corrupt/inaccessible/foreign state is an error, never permission to initialize over it. Queued deployment operations continue independently of SSH once submitted. Keep their operation IDs and inspect status after disconnecting.

The previous local `dzdeploy bootstrap` command remains available for existing account-based installations. Use `dzdeploy agent upgrade <alias>` for management-service upgrades that preserve the running game; see [agent upgrade workflows](operator-workflows.md#agent-upgrades). Bootstrap is not part of fresh-server provisioning. Old non-admin SFTP endpoints continue to work for already enrolled hosts, but agent upgrades require administrative SSH execution.

### Bootstrap prerequisites and missing ssh-keygen.exe

Use elevated **64-bit Windows PowerShell**. A 32-bit shell redirects `System32` to `SysWOW64` and cannot load the LocalAccounts module used by this script. The script detects that condition before account or installation changes and explains how to open the native shell. See Microsoft's [file system redirection](https://learn.microsoft.com/en-us/windows/win32/winprog64/file-system-redirector) and [LocalAccounts](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.localaccounts/get-localuser?view=powershell-5.1) documentation.

The script resolves the actual `sshd` service executable first. A complete existing installation is reused without requiring the Client capability to report installed. Otherwise, it tries both `OpenSSH.Client~~~~0.0.1.0` and `OpenSSH.Server~~~~0.0.1.0` and queries their states again. Microsoft documents separate [Client and Server installation commands](https://learn.microsoft.com/en-us/windows-server/administration/openssh/openssh_install_firstuse?tabs=powershell), and notes that [`Add-WindowsCapability` can fail silently](https://learn.microsoft.com/en-us/powershell/module/dism/add-windowscapability?view=windowsserver2025-ps#-source) when all package sources fail.

If Windows throws, stays `NotPresent`, or leaves required files missing, the same script automatically downloads Microsoft's `10.4.0.0p1-Preview` Win64 ZIP. This explicitly selected Preview release is pinned by its [published SHA-256](https://github.com/PowerShell/Win32-OpenSSH/releases/expanded_assets/10.4.0.0p1-Preview), and every executable and DLL must have a valid Microsoft signature. Extraction rejects unsafe paths and stages files in a protected directory before installation under `%ProgramFiles%\DayZDeploy\OpenSSH\10.4.0.0p1-Preview`. A per-file hash manifest verifies reuse on subsequent runs. Package version updates require updating the script's pin; this installation is not serviced by Windows Update.

The ZIP fallback configures only `sshd`, using LocalSystem, an absolute SFTP path, and file logging under `%ProgramData%\ssh\logs`. It does not execute Microsoft's general installer, change `ssh-agent`, or add a broad installer firewall rule. Existing SSH services with custom command-line arguments or a different service account are rejected without replacing those settings. Existing host keys and authorized keys are retained on retry.

If Windows explicitly reports servicing pending or a reboot required, finish servicing and rerun the script after reboot. Other Windows feature-source failures recover through the ZIP automatically; the operator does not need to fix WSUS or copy another file. If the package cannot be downloaded or verified, the script stops before configuring SSH and reports the failure. Microsoft's [installation troubleshooting](https://learn.microsoft.com/en-us/troubleshoot/windows-server/system-management-components/cant-install-openssh-features) remains useful for the underlying Windows feature problem.

## Routine commands

See [operator workflows](operator-workflows.md) for fleet health and restart history.

```text
dzdeploy tools install
dzdeploy doctor nasdara-staging
dzdeploy check nasdara-staging
dzdeploy deploy nasdara-staging --dry-run
dzdeploy deploy nasdara-staging
dzdeploy deploy nasdara-staging --restart
dzdeploy status nasdara-staging
dzdeploy restart nasdara-staging
dzdeploy rollback nasdara-staging --deployment RETAINED_ID --restart
```

Doctor checks an enrolled host's pinned SSH, SFTP, target/protocol identity, agent heartbeat, and Git eligibility. It is optional after provisioning. Synchronize host/client clocks; heartbeats older than 15 seconds fail.

Deployment fetches `origin/main` and requires clean main equal to that revision, including no untracked non-ignored files. It never commits, merges, switches branches, stashes, resets, or pulls for you. Packages contain exact committed bytes and launcher settings, with a second Git check after validation. Merging alone never deploys.

Dry-run validates locally and reads remote file hashes/state, listing additions, replacements, and removals. Fresh-host previews also report provisioning. It creates no remote packages, requests, backups, installations, or live-file changes, and never persists a new host key. It requires an eligible source checkout.

Packages are uploaded/read-back verified before a request is atomically published. The host independently verifies them, serializes operations, preflights paths/disk/write access, backs up actual preimages, journals changes, applies files, deletes obsolete owned paths, and verifies hashes. Main overwrites live drift after backup. Atomic replacement applies per file; copying while running cannot provide an atomic switch across all configuration files.

Without `--restart`, the running game is left running and activation is reported pending. Installed revision and last managed startup revision are separate. The first deployment should use `--restart` to activate the initially idle service.

Explicit restarts request immediate clean RCon shutdown without a warning countdown. Default timeouts are three minutes for shutdown, 30 for updates, and ten for startup. Stuck games are not force-killed. Every launch runs **updates → reapply desired configuration → launch**, preserving mod order, ports, logging, and BattlEye paths. Startup verifies process identity, hostname, and map through Steam query. RCon remains the clean-shutdown channel; an RCon response is not required to declare query readiness.

Steam server names have a [64-byte UTF-8 buffer including the null terminator](https://partner.steamgames.com/doc/api/ISteamMatchmakingServers#k_cbMaxGameServerName). If the configured hostname exceeds 63 bytes, local Steam-query and public DZSA checks also accept its exact first 63 bytes, as advertised by DayZ. Other prefixes and different names remain errors; the configured name and listener ports are preserved.

All three missions use DayZ's native graceful shutdown after **180 minutes of mission uptime**. The countdown resets each time the mission starts; update and loading time add to the interval between launches. The managed files are:

| Alias | Tracked mission file under its server directory | Installed path relative to `install_dir` |
| --- | --- | --- |
| `nasdara` | `mpmissions/dayzOffline.nasdara/db/messages.xml` | `mpmissions/dayzOffline.nasdara/db/messages.xml` |
| `nasdara-staging` | `mpmissions/dayzOffline.nasdara/db/messages.xml` | `mpmissions/dayzOffline.nasdara/db/messages.xml` |
| `deer-isle` | `empty.deerisle/db/messages.xml` | `mpmissions/empty.deerisle/db/messages.xml` |

Each shutdown entry has `<deadline>180</deadline>`, `<shutdown>1</shutdown>`, and the text `#name will restart in #tmin minutes.` DayZ supplies its native warnings at 90, 60, 45, **30**, 20, 15, **10**, **5**, 2, and 1 minute remaining. These checkpoints are built into DayZ, as described in the [official server-message documentation](https://community.bistudio.com/wiki/DayZ:Server_Messages); no custom warning scheduler is installed.

DayZ owns the warning countdown and graceful shutdown. The corresponding `dzdeploy-<alias>` Windows service stays running, detects the game exit on its two-second supervision loop, runs updates, reapplies the installed configuration, and starts DayZ again. This path does not request RCon shutdown. The batch files are compatibility entry points for starting that service; they do not implement a separate restart loop or delay. No Windows scheduled task is needed.

Automatic relaunch requires an installed release and an unpaused agent. A failed or interrupted startup can leave `paused: true` in the generated protected `state.json`; inspect the failed operation and recover through a managed startup rather than editing that state by hand. `host.json`'s shutdown/update/startup seconds are operation timeouts, not the three-hour timer. An existing enrolled agent must be upgraded to receive newer startup-readiness behavior; copying mission XML alone does not replace the service executable. Deployment/restart authorization and the clean-main gate still apply, and local validation does not establish a successful live shutdown/relaunch cycle.

When `public_query_address` is set on the host, startup also calls the same HTTPS API as the DZSA server-check page: `GET https://dayzsalauncher.com/api/v1/query/IP:QUERY_PORT`. A successful check requires HTTP 200, DZSA status 0, the requested endpoint, DayZ, the configured game port, the deployed hostname and map, and an exact set of expected Workshop IDs. Mod order and display names do not affect the comparison. Missing, unexpected, duplicate, or zero Workshop IDs do not pass. Successful deployment results and state retain the timestamp, check link, server details, and returned mod names/IDs; text and JSON output show that report.

DZSA calls have a ten-second request timeout and a bounded response size. Startup retries at most once every 15 seconds within its existing startup timeout to allow Steam/DZSA discovery to catch up. If public confirmation remains unavailable, the operation reports failure and leaves the locally query-ready game and installed configuration running. An external discovery failure does not trigger a stop or previous-configuration rollback.

`dzdeploy check <alias> [--json]` performs a fresh read-only DZSA check using the installed manifest and live hostname. It does not deploy, restart, run RCon commands, update deployment state, or require clean main. The client YAML's `host.public_query_address` overrides the enrolled host's saved public query address for this check; if neither is set, a public IPv4 SSH address plus the installed query port is used. Private SSH addresses require the public query address in YAML. This command also works with an older enrolled agent; automatic startup verification and persisted reports require the updated Windows agent and its saved public query address. Client YAML changes alone do not reconfigure an already enrolled agent. Copy-only deployment and dry-run do not claim to have verified a new startup, and `--no-wait` still returns after submission.

Operations survive client disconnection. Keep the printed ID and reconnect using:

```text
dzdeploy status nasdara-staging --operation OPERATION_ID
```

`--no-wait` returns after durable submission. `--json` provides structured results. Exit codes: 0 success/submitted, 1 failure, 2 Git eligibility declined. Submission is not completion; inspect the operation instead of resubmitting blindly.

## Ownership, recovery, and validation

The allowlist includes server config, batch compatibility launchers, mission XML/JSON/scripts, VPP SuperAdmins, selected Expansion settings/loadouts/market/traders/quest definitions, and Terje settings. It excludes tools, documentation/audits, logs, storage, player data, ATM balances, and VPP runtime credentials. Traversal, reserved Windows names, symlinks/reparse points, and colliding destinations are rejected.

Nasdara folders remain partial overlays. Deer Isle's `empty.deerisle/` maps to `mpmissions/empty.deerisle/`. Only previously owned paths can be deleted; stock files and persistence are preserved. All three servers disable VPP's password prompt, retain Git-managed SteamID allowlists, and leave live `credentials.txt` files untouched.

Failed writes restore file contents and ownership records. Interrupted journals recover before another launch. Failed recovery preserves its journal and blocks launching; repair disk/access and restart the service, without deleting the journal. Interruptions during updates/startup require inspection and explicit restart. Failed requested startup gets one previous-configuration recovery attempt without repeating updates; game/mod updates and persistence cannot be rolled back by this tool.

Protected `releases/`, `backups/`, `results/`, and `logs/` hold packages, preimages, operation results, and host diagnostics. Retain ten successful releases and ten successful-operation backups, preserving the current installed release. Failed artifacts remain for diagnosis. SSH/BattlEye bootstrap backups remain on the host.

```text
dzdeploy validate all
dzdeploy validate nasdara-staging --offline
go test ./...
go test -race ./...
go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

The Go installer downloads the Windows/Linux release and verifies its SHA-256 digest. Runtime validation invokes the binary directly. `--offline` is local-validation-only; deployments check the release. Missing/failed validators or unexpected help block deployment; warnings are reported.

Run `powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/test-bootstrap-windows.ps1` for the mocked bootstrap suite; it changes only an isolated temporary directory. CI runs it using Windows PowerShell, without provisioning the runner. `go run ./cmd/dzdeploy-package` verifies portable distribution builds. Optional `DZDEPLOY_CHECK_VENDOR_DOWNLOADS=1` Go tests verify actual vendor download signatures without executing installers.

CI also runs `scripts/test-bootstrap-windows-integration.ps1 -DisposableRunner` on disposable Windows Server 2022/2025 GitHub-hosted runners. It simulates unavailable Windows capabilities, downloads the real verified ZIP, starts the real SSH service, authenticates with an ephemeral test key, transfers a file using SFTP, and verifies a rerun preserves the host fingerprint. This test refuses to run on operator workstations and existing game servers. CI results demonstrate runner acceptance; target-server acceptance requires running the script on those servers.

Baseline: 13 checks, no failures, three existing Deer Isle economy warnings. Local validation does not establish runtime behavior. Live rollout is deferred until YAML is populated. On a fresh staging VM verify that the script and one deploy command are sufficient, including in-command Steam prompts. Then verify copy-only PID preservation, clean restart/persistence, update/overlay ordering, managed deletion with stock/player data preserved, rollback, interrupted enrollment/SSH recovery, reboot startup without RDP, password-free VPP for allowlisted admins, and denial for unauthorized players. Then enroll each other host separately.
