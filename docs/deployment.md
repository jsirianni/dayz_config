# Go DayZ deployment

## Build and configure

```text
go build -o .tools/bin/ ./cmd/dzdeploy
```

Windows produces `.tools/bin/dzdeploy.exe`; Linux/macOS produce `.tools/bin/dzdeploy`. Add the directory to PATH or use its full path. The module selects Go 1.27.3. Go is not needed to run the compiled executable. Git is required on the client; SteamCMD and DayZ are required on each Windows host.

`deploy.yaml` is the single user-maintained configuration file. A local placeholder copy is provided; fresh clones should copy `deploy.example.yaml` to `deploy.yaml`. Both that populated file and `.deploy/` are ignored by Git. Fill the three host addresses, SSH sources, paths, Steam account names, and verified listener ports. Unknown fields, duplicate keys, multiple documents, and unsupported aliases/versions are rejected.

| Alias | Source under servers/current | Game port | Query port |
| --- | --- | ---: | --- |
| `nasdara-staging` | `first-light-badlands-nasdara-staging-us-east` | 2901 | 27018 |
| `nasdara` | `first-light-badlands-nasdara-us-east` | 2301 | Discover on host |
| `deer-isle` | `first-light-deer-isle-expansion-us-east1` | 4801 | 27019 |

Nasdara has no explicit query port in Git; discover its actual listener rather than deriving it from the game port. Example RCon ports are configurable choices. Give instances on the same machine separate installations/state directories and nonconflicting ports.

SSH key paths are relative to YAML. Omit `key_file` to use Windows OpenSSH's agent or Unix `SSH_AUTH_SOCK`. Encrypted key files can use `key_passphrase_env: VARIABLE_NAME`. Keep private keys and passwords out of YAML. Git's `serverDZ.cfg` is copied exactly, including its existing game password values.

The nested `host` settings are used by bootstrap; no second user-maintained file is needed. Bootstrap writes private machine-readable `host.json` for the service. Launcher mods/order/game ports remain tracked in `internal/deploy/targets.json` and are packaged from the deployment commit.

## One-time setup, staging first

In the trusted RDP/admin session:

1. Back up the installation/persistence and stop DayZ and its old console manager. Bootstrap refuses a detected game process. Do not leave another supervisor running.
2. Create an ordinary Windows account, such as `dayz-deploy`, with a nonempty password. Do not add it to Administrators. Bootstrap grants service logon and file access; it does not create/elevate the account.
3. Run SteamCMD interactively as that Windows account and authenticate the YAML Steam user, completing Steam Guard. This establishes its cached login. Provision SteamCMD and a complete stock mission before the first managed launch.
4. Copy the Windows executable, populated `deploy.yaml`, and the public SSH key onto the host. Keep the configured relative public-key path valid or override `--public-key`. Do not copy the private key.
5. From an elevated Command Prompt in that folder:

```text
dzdeploy.exe bootstrap nasdara-staging --setup-ssh
```

The account defaults to YAML's `user` as a local account. `--account DOMAIN\user` selects another existing account. The password prompt hides input; `--password-env NAME` supports controlled unattended setup without a password argument. SSH port, allowed source IPs/CIDRs, public key, and host paths come from YAML.

Bootstrap uses native Windows facilities to install OpenSSH if needed, validate/configure key-only SFTP access for the account, scope a firewall rule, and start SSH automatically. It disables the standard broad OpenSSH installation rule; unrelated host/network firewall rules remain under host administration.

It protects instance storage, grants installation/Steam directory access, configures loopback-only RCon, and installs automatic service `dzdeploy-<alias>`. Service recovery restarts a failed agent. The initial service stays idle until a deployment explicitly requests restart. The batch entry points now start/delegate to that service, without a second restart loop.

Bootstrap prints the Ed25519 SSH host fingerprint. On the client, collect the public host key using the trusted session, or use `ssh-keyscan -t ed25519 -p 22 HOST` to obtain a candidate. Compare its SHA-256 fingerprint (`ssh-keygen -lf candidate_file -E sha256`) against trusted bootstrap output before saving the matching key line in YAML's `known_hosts`. Use `[HOST]:PORT` for nonstandard ports. Keyscan alone does not authenticate the host; unknown/changed keys are rejected.

For upgrade/reconfiguration, stop the service first with `sc.exe stop dzdeploy-nasdara-staging`, wait for clean shutdown, then rerun bootstrap using the new binary/YAML. Existing storage and service registration are reused; running-service reconfiguration is declined.

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

Doctor checks pinned SSH, SFTP, host/target/protocol identity, agent heartbeat, and Git eligibility. Synchronize host/client clocks; heartbeats older than 15 seconds fail.

Deployment fetches `origin/main` and requires clean main equal to that revision, including no untracked non-ignored files. It never commits, merges, switches branches, stashes, resets, or pulls for you. Packages contain exact committed bytes and launcher settings, with a second Git check after validation. Merging alone never deploys.

Dry-run validates locally and reads remote file hashes/state, listing additions, replacements, and removals. It creates no remote packages, requests, backups, or live-file changes. It requires an eligible source checkout.

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

The Go installer verifies release SHA-256 digests. For macOS without an upstream binary asset, initial installation builds the released checksum-verified Go module; Go or a preinstalled native `.tools/dzcli/dzcli` is required for that step. Runtime validation invokes the binary directly. `--offline` is local-validation-only; deployments check the release. Missing/failed validators or unexpected help block deployment; warnings are reported.

Baseline: 13 checks, no failures, three existing Deer Isle economy warnings. Local validation does not establish runtime behavior. Live rollout is deferred until YAML is populated. On staging verify copy-only PID preservation, clean restart/persistence, update/overlay ordering, managed deletion with stock/player data preserved, rollback, disconnect recovery, reboot startup without RDP, password-free VPP for allowlisted admins, and denial for unauthorized players. Then enroll each other host separately.
