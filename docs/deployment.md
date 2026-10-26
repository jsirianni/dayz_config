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
| `nasdara` | `first-light-badlands-nasdara-us-east` | 2301 | 27017 |
| `deer-isle` | `first-light-deer-isle-expansion-us-east1` | 4801 | 27019 |

Query ports are explicit in tracked configuration. Existing Nasdara deployments now use 27017 on the next managed restart; update the local YAML to match. Example RCon ports are configurable choices. Give instances on the same machine separate installations/state directories and nonconflicting ports.

SSH key paths are relative to YAML. Omit `key_file` to use Windows OpenSSH's agent or Unix `SSH_AUTH_SOCK`. Encrypted key files can use `key_passphrase_env: VARIABLE_NAME`. Keep private keys and passwords out of YAML. Git's `serverDZ.cfg` is copied exactly, including its existing game password values.

The nested `host` settings drive automatic enrollment; no second user-maintained file is needed. Enrollment writes private machine-readable `host.json` for the service. Launcher mods/order/game ports remain tracked in `internal/deploy/targets.json` and are packaged from the deployment commit. `remote_dir` must be the SFTP spelling of `host.state_dir`, such as `/C:/ProgramData/DayZDeploy/nasdara-staging`.

## Two-step provisioning, staging first

Have the client SSH key and populated local `deploy.yaml` available. The host must have internet access for Windows features, vendor installers, Steam, and the pinned stock mission source. Steam authentication requires an account entitled to the configured downloads. Keep the private SSH key on the client.

**Step 1 — prepare one script on the client.** Make a local copy of `scripts/bootstrap-windows.ps1` under ignored `.deploy/`. Replace `REPLACE_WITH_CLIENT_PUBLIC_KEY` in that copy with the complete single-line contents of your client's `.pub` file. Keep this personalized script out of Git. Never embed a private key.

Copy **only that script** to each Windows server, then run in an elevated Windows PowerShell terminal:

```powershell
.\bootstrap-windows.ps1 -AllowedSource '203.0.113.10/32'
```

The same personalized script works on all three hosts. The source template refuses to configure SSH until a valid key is embedded. `-PublicKeyPath` remains available for older workflows; it is unnecessary for this single-file workflow.

`-UserName` defaults to the existing enabled local `Administrator`; `-SshPort` defaults to 22. `-AllowedSource` accepts explicit IPv4/IPv6 addresses or CIDRs, including multiple entries. Use the same username/port in client YAML. `-WhatIf` previews setup without changes. Built-in Windows PowerShell 5.1 is sufficient; no modules, Go executable, repository, or game installation are needed on the host.

The script installs OpenSSH when absent, preserves host keys and existing authorized keys, configures key-only SSH command execution and SFTP, and scopes a firewall rule to the supplied sources. It disables the default broad OpenSSH rule, validates `sshd_config`, starts SSH automatically, and restores configuration/keys/firewall/service settings on setup failure. Backups remain in protected `%ProgramData%\ssh\dzdeploy-backup-*` folders. Windows servicing failures are reported without altering update policy or forcing a reboot.

**Step 2 — on the client**, run:

```text
dzdeploy deploy nasdara-staging --restart
```

On first connection, paste the trusted SHA256 fingerprint printed by the script when prompted. Alternatively supply it as `--host-key SHA256:...` on this same command. The client verifies the fingerprint before enrollment and saves the matching key in `known_hosts`. Existing mismatched keys are always rejected; `--host-key` cannot overwrite established trust. No manual keyscan or known-host editing is required.

The first deploy uploads and read-back verifies its Windows agent, installs SteamCMD and signed Microsoft Visual C++ prerequisites, creates protected directories, sets up loopback RCon/game firewall rules, and installs service `dzdeploy-<alias>`. The service uses `NT SERVICE\dzdeploy-<alias>`, a password-free Windows virtual account. SteamCMD authentication runs under that identity: any password, Steam Guard code, or mobile approval is requested inside the client command. Secrets travel through SSH and a restricted local named pipe; answers never enter command arguments, YAML, persistent results, or logs. Steam's cached login is protected in the SteamCMD directory and reused.

The service downloads DayZ/mods, supplies the complete mission, reapplies the repository overlays, and launches only when `--restart` was supplied. Missing Nasdara stock missions come from official Central Economy commit `f220b2918b1d79a5055b1ee9be70c0ec1516e876`; existing stock files are preserved. Deer Isle's complete tracked mission is deployed to its installed location. No manual SteamCMD login, account creation, host binary copy, mission copy, or extra bootstrap command is required.

Without `--restart`, initial provisioning and configuration copying complete with DayZ stopped and activation pending. A first `--dry-run` reports planned provisioning without uploading or installing anything. Initial Steam prompts complete before `--no-wait` can return a submitted operation. During later `deploy --restart`, authentication is checked before stopping the game.

Enrollment progress is recorded in protected `enrollment.json`; `status` can report it even before the service has created deployment state. If setup or its SSH connection fails, rerun the deploy command to resume. Existing healthy deployments are reused; corrupt/inaccessible/foreign state is an error, never permission to initialize over it. Queued deployment operations continue independently of SSH once submitted. Keep their operation IDs and inspect status after disconnecting.

The previous local `dzdeploy bootstrap` command remains available for existing account-based installations and explicit upgrades. It is not part of fresh-server provisioning. Old non-admin SFTP endpoints continue to work for already enrolled hosts.

## Routine commands

```text
dzdeploy tools install
dzdeploy doctor nasdara-staging
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

Explicit restarts request immediate clean RCon shutdown without a warning countdown. Default timeouts are three minutes for shutdown, 30 for updates, and ten for startup. Stuck games are not force-killed. Every launch runs **updates → reapply desired configuration → launch**, preserving mod order, ports, logging, and BattlEye paths. Startup verifies process identity, RCon response, hostname, and map through Steam query.

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

Baseline: 13 checks, no failures, three existing Deer Isle economy warnings. Local validation does not establish runtime behavior. Live rollout is deferred until YAML is populated. On a fresh staging VM verify that the script and one deploy command are sufficient, including in-command Steam prompts. Then verify copy-only PID preservation, clean restart/persistence, update/overlay ordering, managed deletion with stock/player data preserved, rollback, interrupted enrollment/SSH recovery, reboot startup without RDP, password-free VPP for allowlisted admins, and denial for unauthorized players. Then enroll each other host separately.
