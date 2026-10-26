# Repository Instructions

This repository contains DayZ server configuration. For any task that inspects or modifies DayZ configuration files, use the repo-local `dzcli` skill as the default path.

Before changing supported DayZ config files:

1. Read and follow `skills/dzcli/SKILL.md`.
2. Run `go run ./cmd/dzdeploy tools install` from the repository root to install or update the repo-local `dzcli` binary. The legacy `.\skills\dzcli\scripts\install-dzcli.ps1` remains supported.
3. Set `$dz = Join-Path (Get-Location) ".tools\dzcli\dzcli.exe"` and invoke commands as `& $dz ...`.
4. Prefer `dzcli get`, `create`, `update`, `delete`, or `fix` commands over direct edits for supported files.
5. Preview mutations with `--dry-run` before applying them.
6. Use direct file edits only when `dzcli` does not expose a sufficient command for the required change.

Run baseline validation before edits when practical. If validation already fails, preserve the output and compare it with the post-change validation result.

Always run the matching `dzcli validate ...` command after modifying DayZ config files, whether the change was made by `dzcli` or by direct edit:

- `serverDZ.cfg`: `dzcli validate server <serverDZ.cfg>`
- `cfggameplay.json`: `dzcli validate gameplay <cfggameplay.json>`
- Economy XML or mission economy folders: `dzcli validate economy <mission-root-or-file>`
- XML trees touched directly: `dzcli validate xml <path>`
- `cfgweather.xml`: `dzcli validate weather <cfgweather.xml>`
- `init.c`: `dzcli validate init <init.c>`
- Expansion AI JSON/settings/loadouts/patrols: `dzcli validate expansion ai <server-or-repo-root>`

Do not commit downloaded tool binaries. The installer writes them under `.tools/dzcli/`.

## Deployment

Use the Go `dzdeploy` CLI; see `docs/deployment.md`. Build with `go build -o .tools/bin/ ./cmd/dzdeploy`. The Windows executable is `.tools/bin/dzdeploy.exe`.

Only Windows and Linux clients are supported. Build/test/package those platforms; the host agent runs only on Windows.

- Host configuration is `deploy.yaml` (ignored); `deploy.example.yaml` is its template. Do not commit populated configuration, private keys, credentials, or binaries. The deployment client's public key is intentionally embedded in `scripts/bootstrap-windows.ps1` at the user's request.
- Fresh Windows servers use two provisioning steps: copy only `scripts/bootstrap-windows.ps1` to each host and run it as Administrator with the allowed source network, then `dzdeploy deploy <alias> [--restart]`. The tracked script must already contain the deployment client's actual public key. Do not replace it with a placeholder or require copying a separate public-key file to the host. Deploy automatically enrolls the host and handles prerequisites/Steam prompts. Do not require manual account creation, SteamCMD login, mission copying, or a separate bootstrap command.
- First deploy automatically trusts and saves an unknown SSH host key. Never require the operator to retrieve or paste a host fingerprint. Preserve rejection of changed known keys; explicit `--host-key` remains optional. Missing Steam account names are requested inside deploy, alongside any required Steam authentication.
- Deploy only when the user requests a named target: `nasdara`, `nasdara-staging`, or `deer-isle`. Editing/merging does not itself authorize deployment.
- For enrolled hosts, run `dzdeploy doctor <alias>` and `dzdeploy deploy <alias> --dry-run`, inspect the result, then run `dzdeploy deploy <alias>`. Fresh-host dry-runs preview automatic provisioning; doctor requires enrollment. Add `--restart` only if the user explicitly requests a restart.
- Deployment requires clean `main` equal to freshly fetched `origin/main`. Do not bypass the gate, auto-commit, auto-merge, reset, or stash changes to make it pass.
- Use `dzdeploy validate <alias|all>` for local validation. `--offline` explicitly uses cached dzcli for local checks; it is prohibited for deployment.
- Main controls managed configuration, including server passwords and admin allowlists. Live drift is backed up and overwritten; only previously managed obsolete paths are deleted. Persistence and runtime credentials are excluded.
- Preserve operation IDs. On connection loss, inspect `dzdeploy status <alias> --operation <id>` instead of blindly resubmitting.
- Report installed revision, last managed startup revision, and restart pending separately. Live acceptance requires actual host access.
