# pvecli

Independent Go CLI for Proxmox VE 9.x. It manages QEMU VMs, deploys full clones
through authenticated SSH readiness, and builds Ubuntu 24.04 / Windows Server
2022/2025 templates from installation ISOs. Supported clients are Windows amd64
and Linux amd64. Existing-VM hardware management is independent of guest OS.

This is a separate Go module. It has no dependency on the repository's other Go
programs, their configuration, Git state, or deployment machinery. Run build and
test commands from this directory. Go 1.27.3 is selected by this module.

## Build and package

```text
go build -buildvcs=false -o dist/ ./cmd/pvecli
go test ./...
go vet ./...
go run ./cmd/pvecli-package --os windows --version dev
go run ./cmd/pvecli-package --os linux --version dev
```

Archives under `dist/` contain the executable, this runbook, acceptance checklist,
examples, and a SHA-256 manifest. Each archive also has an adjacent checksum file.
The executable needs no Go installation or Git checkout. Packer and ISO authoring
utilities are needed only for template creation.

## Connections and credentials

Copy `examples/config.example.yaml` to the `pvecli/config.yaml` file under your OS
user configuration directory, or pass `--config /path/to/config.yaml`. Windows
normally uses `%APPDATA%`; Linux normally uses `$XDG_CONFIG_HOME` or `~/.config`.
Select a context with `--context lab` or the configuration's `default_context`.

Set the environment variable named by `token_secret_env` to the API token secret.
Do not put the value in YAML or command arguments. Use a dedicated Proxmox user
and privilege-separated API token. Its effective permissions are the intersection
of the user and token ACLs. A practical lab setup grants VM administration on the
dedicated pool, clone/audit rights on source templates, allocation rights on the
selected storage, and bridge/VLAN use rights. Review each method's permissions in
your installed Proxmox API viewer rather than granting root access.

Readiness additionally needs guest-agent network inspection and file-read access
to the target VM's **public** SSH host key. Current PVE 9 packages expose
`VM.GuestAgent.Audit` and `VM.GuestAgent.FileRead`; older 9.x packages may group
guest-agent access under `VM.Monitor`. Template builds additionally require ISO
upload/delete, VM creation/configuration, guest shutdown, and template conversion.
`doctor` reports effective permissions; it does not reserve resources or prove
that a subsequent allocation will succeed.

All API connections verify HTTPS using the OS trust store. Install the cluster
CA in that store or use a publicly trusted cluster certificate. This also covers
Packer's API connections. There is no TLS verification bypass flag.

Guest SSH uses an OpenSSH/PEM private key file, resolved relative to the deployment
specification. For encrypted keys set `passphrase_env` to an environment-variable
name. No private key is uploaded. The selected NIC's MAC identifies the correct
guest-agent interface. The host key is obtained through the authenticated API and
pinned for the SSH handshake, not accepted through network key scanning.

## Inventory and existing VMs

```text
pvecli doctor
pvecli nodes list
pvecli storage list --node pve1
pvecli networks list --node pve1
pvecli vm list
pvecli vm show 9201
pvecli template list
pvecli template show 9100
pvecli vm update 9201 --cores 4 --memory-mib 8192 --dry-run
pvecli vm update 9201 --cores 4 --memory-mib 8192
pvecli vm update 9201 --onboot=false
pvecli vm nic update 9201 --interface net0 --bridge vmbr0 --vlan 20 --dry-run
pvecli vm nic update 9201 --interface net0 --vlan none --enabled=true
pvecli vm disk resize 9201 --disk scsi0 --size 100GiB --dry-run
pvecli vm start 9201
pvecli vm shutdown 9201
pvecli vm reboot 9201
pvecli vm stop 9201 --force
```

Mutations support `--dry-run`. Previews read state without submitting tasks,
executing guest commands, creating journals, uploading media, or launching builds.
CPU and NIC patches preserve unrelated property subfields. Updates use the
configuration digest to reject intervening changes. Saved configuration and
pending changes are reported separately; hardware edits never restart the VM.

NIC edits patch existing devices only. Disk resize accepts an absolute K/M/G/T
size, expands only, and leaves guest partitions/filesystems unchanged. No VM
deletion, snapshot, migration, disk attachment/removal, container management,
WinRM, or application deployment commands are included.

## Deploy a template

Copy `examples/deploy.example.yaml` and select explicit VMIDs, context, node,
pool, storage, hardware, network, and SSH key. Templates must be on the target
node, have a Cloud-Init drive, enable a working QEMU guest agent, support public
key injection, and generate fresh SSH host keys on each clone. For Windows use
`guest.os: windows-2022` or `windows-2025` and a prepared Cloudbase-Init template.
Templates built by this CLI use `pveadmin` on both operating systems.

```text
pvecli vm deploy --file deploy.yaml --dry-run
pvecli vm deploy --file deploy.yaml --timeout 45m
pvecli vm ready 9201 --file deploy.yaml --timeout 5m
```

Deployment validates inputs and resource visibility, creates a same-node full
clone, applies hardware/network/initialization, starts it, and verifies a remote
SSH command. It returns the verified SSH endpoint and host-key fingerprint.
DHCP requires one usable IPv4 address on the selected NIC; static IPv4 must match
the requested address. IPv6 and cross-node clone placement are deferred.

## Create templates from ISO

```text
pvecli tools install --dry-run
pvecli tools install
pvecli doctor --template-build
pvecli template build --file template.yaml --dry-run
pvecli template build --file template.yaml --timeout 2h
```

The installer downloads Packer 1.16.2 and Proxmox plugin 1.2.4 over HTTPS, verifies
their release SHA-256 checksums, and installs them in the user's cache. Use
`--tools-dir` to select another private cache. Install `xorriso` on Linux or the
Windows ADK's `oscdimg` utility on Windows and expose it on PATH.

Use the Ubuntu or Windows template example. OS and VirtIO media must be supplied
as existing Proxmox `storage:iso/name.iso` references, or local files with verified
`sha256` values. Existing storage references are trusted operator inputs; their
contents are not remotely hashed by this CLI. Windows media licensing and
activation remain operator responsibilities. The initial Windows recipe targets
English-language, unattended-compatible Server media; set the exact image name.
For Server 2025 select `os: windows-2025` and matching ISO/image values. Supply a
tested Cloudbase-Init x64 MSI HTTPS URL and checksum. Example placeholders must
be replaced before use.

The designated build network needs DHCP, outbound package/vendor access, and
client-to-guest SSH reachability. Installation is unattended. A native local
worker verifies the guest SSH host key through the guest agent, provisions it,
generalizes it, verifies a sealing marker, and shuts it down. Packer then converts
it into a template and removes installation drives. The template records recipe
version/OS provenance. The workflow uses temporary build credentials and removes
them and machine/SSH identities before sealing.

Template builds are foreground operations and cannot be automatically resumed.
Failed provisioning builds may be retained for inspection. Temporary local keys
and answer files are removed; cleanup of operation-owned answer ISO media is
attempted, with failures reported as residual artifacts. Power loss can prevent
cleanup: inspect the journal and protect/remove any reported answer media, which
may contain the temporary Windows installation password. Existing VMs/templates
are never overwritten or automatically rebuilt.

## Recovery, JSON, and state

```text
pvecli operation status OPERATION_ID --json
pvecli operation resume OPERATION_ID
```

Journals live under the app's OS user configuration directory in `state/`,
overridable with `--state-dir`. Windows state has a protected user/SYSTEM ACL;
Linux uses owner-only directory/file permissions. Local VM locks prevent competing
CLI operations, and journals preserve intent and Proxmox task IDs atomically.

Commands wait by default. Ctrl+C/timeouts do not cancel a submitted Proxmox task.
After a connection failure inspect status before resuming. Native deployment
resume requires the same context identity, input digest, and remote ownership
marker. The SSH identity must match the submitted public key. Completed phases
are not repeated. If a submission response was lost
before its task ID was recorded, the CLI refuses another mutation and requests
manual task/state inspection. A task that failed is not automatically resubmitted.
There is no detached workflow daemon or background completion guarantee.

`--json` emits one result object on stdout, with progress on stderr. Results
include `apiVersion`, status, operation/VM/task identities, phase, pending changes,
and SSH details where available. Exit codes: 0 success/preview, 1 execution
failure, 2 invalid input/preflight rejection, 3 timeout/unresolved outcome.
`--timeout` defaults to 50 minutes. Operation status reports stored workflow state
alongside current task observations; use resume to finish a confirmed deployment.

## Validation

```text
go test ./...
go vet ./...
go test -race ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
python scripts/standalone.py
```

Tests use local TLS API fixtures, an actual local SSH server, and fake build
processes. To additionally run the pinned Packer parser against all generated
recipes set `PVECLI_TEST_PACKER` to its binary and `PVECLI_TEST_PLUGINS` to its
plugin directory before testing. This invokes `packer validate` only.

Real Proxmox/guest acceptance is separate and opt-in. Follow `docs/acceptance.md`
before treating the template recipes as accepted for your installation media and
cluster. Passing unit tests and Packer validation do not establish guest boot,
Sysprep, network, or clone identity behavior on a real host.
