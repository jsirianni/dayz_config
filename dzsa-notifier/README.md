# dzsa-notifier

Independent Linux service that periodically submits DayZ query endpoints to
DZSA's server-check API. It starts checking immediately, then every five minutes
by default. It needs no SSH access to DayZ, Steam account, RCon, or deployment
manifest. It does not change game configuration, ports, or processes.

## Contents

- [Install a package](#install-a-package)
- [Configure from Windows](#configure-from-windows)
- [Configuration and environment](#configuration-and-environment)
- [Run a container](#run-a-container)
- [Logs and metrics](#logs-and-metrics)
- [Failures and scheduling](#failures-and-scheduling)
- [Build, test, and release](#build-test-and-release)

## Install a package

Download the matching Linux amd64 or arm64 DEB/RPM and `checksums.txt` from this
repository's GitHub release. Verify it with `sha256sum --ignore-missing -c
checksums.txt`, then install with `sudo apt install ./dzsa-notifier_*.deb` or
`sudo dnf install ./dzsa-notifier_*.rpm`. An APT/YUM repository is not provided.

Installation provides `/usr/bin/dzsa-notifier`, the systemd unit, and
`/etc/dzsa-notifier/config.example.yaml`. It does not enable/start the service
until you configure it. The process uses a systemd dynamic user, no Linux
capabilities, and filesystem protection. Running the daemon as root is rejected.
Offline `--check-config` and `--version` are permitted administratively.

Upgrading a package preserves your active YAML and enablement and restarts an
already active notifier. Removing the package stops/disables its service and
leaves your operator-managed YAML and configuration backups in place.

## Configure from Windows

The Linux host must already have the package, OpenSSH, systemd, coreutils,
util-linux (`flock`), and procps (`ps`). Use root SSH access or an administrative
account with noninteractive sudo. This script does not provision Linux or install
the application package.

Keep the authoritative YAML in the repository's ignored
`.deploy/dzsa-notifier.yaml`. Its initial local copy lists the three existing
public query endpoints. Add/remove entries there and rerun from this checkout:

```powershell
.\dzsa-notifier\scripts\Configure-Notifier.ps1 -SSHHost linux.example.com -User admin
```

Optional parameters: `-Port`, `-IdentityFile`, `-KnownHostsFile`, and `-ConfigPath`.
The default identity is `~/.ssh/id_ed25519`; the default known-hosts file is
`~/.ssh/known_hosts`. Windows PowerShell 5.1 and PowerShell 7 are supported.

The script trusts an unknown host on the first real connection and rejects a
changed key. `-WhatIf` uses strict existing trust and only inspects service state
and the current config hash; it cannot enroll an unknown host. No fingerprint
retrieval or additional key copy is required.

Configuration transactions are serialized with a host lock. YAML is uploaded to
protected staging, checksum-checked, validated by the installed executable, and
atomically installed. Existing YAML is backed up beside the file. The script
enables/starts the notifier and restarts an active notifier only when YAML changes.
Activation failure restores the prior file, enablement, and running state.
Identical reruns keep an active process running. This only manages the notifier.

The SSH workflow requires YAML to be authoritative. It rejects `DZSA_NOTIFIER_`
overrides in systemd's manager/unit environment or its optional
`/etc/dzsa-notifier/environment`, and additional environment files, rather than
silently applying configuration that would be shadowed. Environment configuration
remains supported for installations managed without this script.

## Configuration and environment

```yaml
interval: 5m
request_timeout: 10s
metrics_listen: "127.0.0.1:9090"
endpoints:
  - "203.0.113.10:27016"
  - "203.0.113.10:27019"
```

Default path: `/etc/dzsa-notifier/config.yaml`; select another using `--config`.
Precedence is defaults, then YAML, then environment. Supported overrides:

| Variable | Value |
| --- | --- |
| `DZSA_NOTIFIER_INTERVAL` | Go duration, default `5m` |
| `DZSA_NOTIFIER_REQUEST_TIMEOUT` | Go duration, default `10s` |
| `DZSA_NOTIFIER_METRICS_LISTEN` | TCP bind address, default `127.0.0.1:9090` |
| `DZSA_NOTIFIER_ENDPOINTS` | Comma-separated list replacing the entire YAML list |

The default file may be absent if endpoints are supplied through the environment.
An explicitly selected missing file always fails. Unknown YAML fields, multiple
documents, empty endpoint lists, duplicate endpoints, private/non-IPv4 addresses,
invalid ports, and nonpositive durations are errors. Use public **query ports**,
not game ports. DNS names and IPv6 query endpoints are not supported.

```sh
dzsa-notifier --check-config --config ./config.yaml
dzsa-notifier --once --config ./config.yaml
sudo systemctl enable --now dzsa-notifier
```

`--check-config` performs no network requests. `--once` does not start a metrics
listener; it returns nonzero on failed or deferred checks. `--version` prints the
embedded release version. These three modes are mutually exclusive; malformed
flags and unexpected positional arguments exit 64. `--help` writes usage to stderr
and exits successfully. Configuration changes require a notifier restart.

## Run a container

The image at `ghcr.io/jsirianni/dzsa-notifier:VERSION` is a single amd64/arm64 OCI
index. Stable releases also update `latest`. The digest-pinned distroless Debian
13 nonroot base includes HTTPS trust roots and no shell/package manager. The
application runs as UID/GID 65532 and needs no writable volume or privileges.
This private repository's GHCR package may require registry authentication.

Run on an already configured rootless Podman or Docker engine:

```sh
podman run -d --name dzsa-notifier --restart=unless-stopped \
  --read-only --cap-drop=ALL --security-opt=no-new-privileges \
  -p 127.0.0.1:9090:9090 \
  -v "$PWD/config.yaml:/etc/dzsa-notifier/config.yaml:ro" \
  -e DZSA_NOTIFIER_METRICS_LISTEN=0.0.0.0:9090 \
  ghcr.io/jsirianni/dzsa-notifier:VERSION
```

Alternatively omit the mount and set `DZSA_NOTIFIER_ENDPOINTS` and other overrides.
The mounted file must be readable by UID 65532. Configure SELinux mount labeling
as required by your host. No systemd runs inside the container. Change its YAML or
environment and restart/recreate the container; the SSH script manages native
systemd installations only.

## Logs and metrics

Logs are JSON on stdout: `journalctl -u dzsa-notifier -f` or
`podman logs -f dzsa-notifier`. Each attempt reports its endpoint and either
server name/mod count or error. Each completed cycle reports attempt/failure counts.

Prometheus scrapes `http://127.0.0.1:9090/metrics` by default. Set
`metrics_listen` to a reachable interface for a remote Prometheus instance; the
HTTP endpoint has no authentication. Scraping never sends notifications.

Per-endpoint metrics (the `endpoint` label contains the configured address):

- `dzsa_notifier_attempts_total`
- `dzsa_notifier_failures_total`
- `dzsa_notifier_last_attempt_success` (0 before the first attempt)
- `dzsa_notifier_last_success_timestamp_seconds` (0 until success)
- `dzsa_notifier_request_duration_seconds` (histogram)

Standard Go and process metrics are also included. A useful freshness alert is
`time() - dzsa_notifier_last_success_timestamp_seconds > 900` for a five-minute
interval. Deferred requests are not counted as attempts.

## Failures and scheduling

Every notification is a verified HTTPS GET to
`https://dayzsalauncher.com/api/v1/query/IP:PORT`. Success requires HTTP 200, DZSA
status zero, and a matching DayZ endpoint. This confirms DZSA's returned result;
it does not guarantee continued visibility or introduce a separate registration
API. Returned responses are limited to 1 MiB; redirects are not followed.

Checks are sequential. Normal failures do not block the remaining servers and
are retried next cycle. `Retry-After` (seconds or HTTP date) defers **all** requests
until a scheduled cycle after the deadline. Processing resumes with the next
endpoint, avoiding starvation. Slow cycles skip missed timer slots; they never
overlap or create a backlog. SIGTERM/SIGINT cancels active requests and stops the
metrics listener. No persistent state is needed.

DZSA/network outages do not stop the daemon. Invalid configuration exits 64,
root execution exits 77, and listener/runtime failures exit 1. Systemd does not
restart configuration/permission errors and restarts unexpected failures after
ten seconds. Readiness means configuration and metrics binding succeeded; it
does not depend on external DZSA availability.

## Build, test, and release

This is a separate Go module, independent of dzdeploy and pvecli.

The command entrypoint only handles process signals and exit wiring.
`internal/cli` owns argument parsing, configuration and permission checks, mode
selection, and exit codes. `internal/service` owns the metrics listener, systemd
notifications, worker lifetime, and graceful shutdown. `internal/notifier` owns
the DZSA client, notification scheduling, configuration schema, and instrumentation.
CLI parsing uses an independent flag set per invocation. Lifecycle failures close
the metrics listener and cancel/join the worker before returning to the supervisor.

Run commands in this directory using Go 1.27.3 and GoReleaser OSS **v2.19.0**:

```sh
go test -race ./...
go vet ./...
goreleaser check
goreleaser release --snapshot --clean --skip=docker
```

Remove `--skip=docker` on a Linux Docker/buildx host to build and load both snapshot
images. Snapshot tags are `0.0.0-snapshot-amd64` and `0.0.0-snapshot-arm64`.
Production images copy the exact GoReleaser-built static binaries.

CI exercises DEB/systemd/real SSH on Ubuntu 24.04, RPM/systemd on disposable
AlmaLinux 9, both image architectures (arm64 via QEMU), and rootless Podman.
Integration traffic goes only to a local HTTPS fixture using a generated test CA;
it never notifies the live servers. Windows can run portable core unit tests and
cross-build Linux packages; Linux acceptance belongs to CI or an actual Linux host.

Repository `v*` semantic-version tags are reserved for this app (initial release
`v0.1.0`). The free GoReleaser edition runs in this subdirectory; no Pro monorepo
features are used. On tags, CI first runs acceptance tests and requires the tagged
commit to be on `main`, then publishes GitHub release assets/checksums and the
GHCR multi-architecture image. Prereleases do not update `latest`. Ordinary pushes
and PRs upload snapshot packages without publishing releases or deploying hosts.
