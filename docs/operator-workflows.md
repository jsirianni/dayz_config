# dzdeploy operator workflows

Run these commands from Windows or Linux using the existing ignored `deploy.yaml`.
Read-only operations work from any branch and do not change the host or use RCon.
Existing enrollment establishes SSH trust; changed host keys remain errors.

## Health and history

```text
dzdeploy health all
dzdeploy health nasdara --json
dzdeploy history all --since 24h
```

Health checks actual service/process identity, agent freshness, supervision, UDP
listeners, local A2S query identity, and fresh public DZSA discovery/mods. Installed
revision, last startup revision, and pending activation are reported separately.
Fleet output includes every configured target even when another target fails.
Nonzero exit means at least one required health check failed. Starting/updating
and unreachable servers are distinguished from a healthy running game.

History combines durable lifecycle events on newer agents with retained operation
results and explicit DayZ termination markers. It pairs clean exits with subsequent
readiness evidence within 45 minutes; a missing marker never proves a clean exit.
Historical log timestamps use file modification time and are identified as such.
The report explicitly marks incomplete history and bounded scans (2,000 events,
200 operations, and 24 RPT tails). Missing history does not mean no restarts occurred.

Existing agents remain inspectable through SFTP and a fixed read-only Windows
PowerShell collector over SSH. A restricted SFTP-only account cannot provide native
process inspection; the command reports that limitation instead of trusting saved
state. Newer agents write immutable UTC event files and advertise capabilities in
their fresh heartbeat. This does not automatically upgrade an existing host.

## Logs and diagnostics

```text
dzdeploy logs deer-isle --kind rpt --tail 200
dzdeploy logs nasdara --follow
dzdeploy diagnose all --since 24h
```

Log kinds are `console`, `updates`, `agent`, `rpt`, `script`, and `crash`. Reads are
restricted to recognized files, cap each excerpt at 2 MiB, and redact sensitive
lines and private-key blocks before output. Follow mode emits complete redacted
lines, notices rotation/truncation, and stops on Ctrl+C. Default log tail: 200 lines.

Diagnostics save ZIP bundles under ignored `.deploy/diagnostics/` unless `--output`
is provided. Bundles include health/history, operation evidence, redacted excerpts,
and grouped error summaries. Per-session logs are identified as current/historical
when native process session identity is available; append-only logs are marked mixed.
The latest three files per kind are considered, with 2,000 lines per excerpt and
25 MiB total excerpt content. Missing and truncated sources are reported explicitly.
Configuration, credential files, saved worlds, and memory dumps are not collected.

## Guarded hard restarts

```text
dzdeploy restart nasdara --hard --dry-run
dzdeploy restart nasdara --hard
dzdeploy deploy nasdara --restart --hard
dzdeploy rollback nasdara --deployment RETAINED_ID --restart --hard
```

`--hard` explicitly authorizes stopping the identified managed game without RCon.
The agent pauses supervision, verifies its executable/PID/creation token using the
same Windows handle used for termination, rejects other processes in that
installation, and waits at most 30 seconds. It then runs normal updates, reapplies
configuration, and verifies startup. Graceful stop behavior remains the default;
there is no automatic escalation. Existing listener ports are preserved.

The client requires a fresh `hard-stop.v1` agent capability before submission.
Older agents require an explicit upgrade; these commands do not perform one.
Operation IDs and idempotent results survive SSH disconnects. Inspect an interrupted
operation using `status --operation` instead of blindly resubmitting it. A failed
or interrupted stop/start can leave supervision paused for explicit recovery.
