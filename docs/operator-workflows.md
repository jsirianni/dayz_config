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

Log kinds are `console`, `updates`, `subprocess`, `agent`, `rpt`, `script`, and `crash`. Reads are
restricted to recognized files, cap each excerpt at 2 MiB, and redact sensitive
lines and private-key blocks before output. Follow mode uses a raw byte cursor,
buffers incomplete lines before redaction, and retains private-key redaction state
between reads. Repeated identical lines remain separate occurrences, including
lines with identical redacted text. `--tail` limits the initial display; following
then reads appended data in bounded batches. Rotation/truncation is detected using
the source path, modification metadata, and raw-byte continuity anchors because
SFTP does not expose a portable file ID. An intervening replacement with identical
bytes and metadata cannot be distinguished. Ctrl+C stops collection and closes
remote resources. Default log tail: 200 lines.

New host agents capture subprocess output as JSON Lines, with exactly `level`,
`message`, and `timestamp`. stdout uses `info`, stderr uses `error`; the level
identifies the stream and is not a diagnosis of message severity. Timestamps are
capture times from `time.Now().UTC()` in RFC3339Nano form ending in `Z`. Message
text excludes its LF/CRLF terminator. A final unterminated line is flushed on exit.
Each stream preserves its own order; ordering between streams follows capture.
Output is treated as UTF-8 text; invalid byte sequences use Go JSON replacement.
Lines exceeding the 1 MiB buffer are split without dropping text. Very small log
size settings can require smaller chunks to accommodate JSON escaping/metadata.

| Kind | File in the protected host logs directory | Rotation configuration |
| --- | --- | --- |
| `console` | `server-console.log` | `host.logging.console` |
| `updates` | `updates.log` (SteamCMD updates) | `host.logging.steamcmd` |
| `subprocess` | `subprocess.log` (installers/admin helpers) | `host.logging.subprocess` |

Each log defaults independently to `max_size_mb: 50`, `max_backups: 5`,
`max_age_days: 30`, and `compress: true`. Size, count, and age must be positive;
zero/omitted numeric settings select defaults, not unlimited retention. Set
`compress: false` explicitly to disable gzip. Lumberjack uses UTC backup names.
Its count/age cleanup runs when opening or rotating logs, not on an idle timer.
The nominal maximum is six 50 MiB files per kind before compression; temporary
rotation/compression files and delayed cleanup mean this is not a filesystem quota.
Legacy archives can exceed the configured file size until retention removes them.
DayZ's own RPT/script/crash files are separate and are not rotated by this feature.

`logs` reads/follows the active file only. Follow uses bounded polling and can
miss data if multiple rotations occur between polls. `diagnose` includes the
active file and recent recognized timestamped `.log`/`.log.gz` backups, within
the existing three-files-per-kind and bundle limits. Gzip input and expanded work
are limited to 64 MiB per archive; oversized/corrupt archives are reported as
skipped. Archives use the same line redaction as active logs. JSON messages are
decoded before redaction, with valid JSON preserved for recognized bounded
records; oversized/unframeable lines retain the existing redaction markers.
Error summaries group by message without the capture timestamp.

An old plain-text active file (or a file with a torn final record) is archived
before JSON is appended. Historical contents and timestamps are not rewritten.
Readers continue to accept legacy text. Host settings load at process startup;
changing them requires guarded host configuration/service maintenance. Existing
host files without logging settings receive defaults. Deploy does not silently
change an enrolled host's settings or upgrade its binary.

Capture initialization must succeed before a child starts. Runtime write failures
are reported after draining its output, so a full disk does not block or force-kill
the child. Console failures appear in agent status/error evidence; stopping a game
waits for output drainage within the shutdown timeout. A crash of the agent itself
can still lose buffered/in-flight pipe output. Raw interactive Steam authentication
transcripts and answers never enter these logs. Validator output already stored in
operation results and machine-readable subprocess results are not duplicated.


Diagnostics save ZIP bundles under ignored `.deploy/diagnostics/` unless `--output`
is provided. Bundles include health/history, operation evidence, redacted excerpts,
and grouped error summaries. Per-session logs are identified as current/historical
when native process session identity is available; append-only logs are marked mixed.
The latest three files per kind are considered, with 2,000 lines per excerpt and
25 MiB total excerpt content. Missing and truncated sources are reported explicitly.
Configuration, credential files, saved worlds, and memory dumps are not collected.
Partial diagnostic reports returned after cancellation or an output/write failure
are redacted too; failed bundle creation never exposes raw collected evidence.

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

## Saved-world backups

```text
dzdeploy backup create nasdara --stop --dry-run
dzdeploy backup create nasdara --stop
dzdeploy backup list nasdara
dzdeploy backup verify nasdara --backup BACKUP_ID
dzdeploy backup download nasdara --backup BACKUP_ID --output saved-world.zip
dzdeploy backup restore nasdara --backup BACKUP_ID --stop --dry-run
dzdeploy backup restore nasdara --backup BACKUP_ID --stop
```

Backups contain the active mission/world, profiles and mod runtime data, server
configuration, and managed compatibility files. Game/Workshop binaries, logs,
dumps, caches, and deployment/SSH/Steam authentication state are excluded. Archives
can contain server passwords and mod credentials: host directories and downloaded
files are private to the service/operator, Administrators, and SYSTEM on Windows
(owner-only permissions on Linux). They are not encrypted.

The default destination is protected host `state_dir/snapshots/`; download is
explicit. Backups are streaming ZIP archives with a versioned file/hash manifest
and an archive checksum. Interrupted/unverified archives are not listed as valid
backups. The latest ten verified normal backups are retained by creation time.
Automatic cleanup never removes restore safety backups beneath `snapshots/safety/`.

A running game requires `--stop`, which explicitly authorizes a guarded hard stop
without RCon or an extra save request. The snapshot copies persisted on-disk data
after termination; unsaved in-memory changes are not captured. A running game is
resumed without Steam updates; a stopped game stays stopped. Pending activation
or an unknown prior startup revision must be resolved before running maintenance.
Disk space is checked before stopping the game.

Restore verifies and stages the chosen archive before downtime, creates a fresh
safety snapshot, journals all changed preimages, replaces runtime files, removes
obsolete runtime files, and reapplies current managed configuration. The archived
configuration does not roll back Git-managed settings. Game and mod binaries are
not downgraded; inspect the recorded provenance when restoring older world data.
Managed-file exclusions match Windows paths without regard to case, including
settings removed by the currently installed release.
Safety snapshots can also be verified, downloaded, or selected for another restore
by their listed IDs.

Write failures recover preimages. An interrupted restore is recovered before the
agent permits another launch; recovery failures retain their journal and block
launches. Successful interruption recovery leaves supervision paused for inspection
and an explicit managed restart. Do not delete the recovery journal to bypass it.
Rollback recovers file preimages before pruning introduced empty directories.
Unjournaled files such as startup logs are preserved, along with their directories,
and do not block recovery of the saved world and mod data.
Use `status <alias> --operation ID` after disconnects. `--no-wait` submits a durable
create/restore operation; `--dry-run` performs remote reads only.

Creation/restoration require fresh `backup.v1` and `hard-stop.v1` capabilities.
Listing, verification, and download use SSH/SFTP reads without a running agent.
These operations do not upgrade an installed agent automatically.
