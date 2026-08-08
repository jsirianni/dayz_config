# Repository Instructions

This repository contains DayZ server configuration. For any task that inspects or modifies DayZ configuration files, use the repo-local `dzcli` skill as the default path.

Before changing supported DayZ config files:

1. Read and follow `skills/dzcli/SKILL.md`.
2. Run `.\skills\dzcli\scripts\install-dzcli.ps1` from the repository root to install or update the repo-local `dzcli` binary.
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
