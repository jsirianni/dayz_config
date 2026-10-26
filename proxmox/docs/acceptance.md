# Opt-in lab acceptance

Run only against an explicitly selected non-production PVE 9.x context and lab
pool. Reserve unused template and clone VMIDs and supply installation media,
storage/bridge names, API permissions, trusted TLS, and a client SSH identity.
No CI job automatically connects to a Proxmox host.

For Ubuntu 24.04 and each of Windows Server 2022 and 2025:

1. Run doctor and preview the template specification. Confirm no VM, media, or
   guest changes occur during the preview.
2. Build from ISO. Confirm unattended installation, native SSH provisioning,
   successful generalization/shutdown, template conversion, recipe metadata,
   and removal of temporary answer media.
3. Deploy two full clones with distinct VMIDs and different requested settings.
   Test DHCP and static IPv4 separately. Confirm authenticated SSH execution.
4. Confirm the clones have different SSH host keys and OS machine identities
   (`/etc/machine-id` on Ubuntu, machine SID on Windows). Verify requested
   hostname, NIC MAC/address, and public key injection. Confirm no shared host
   private keys, installer passwords/autologon, or temporary build authorization
   remain. On Windows confirm pveadmin access and its authorized-key ACL.
5. Exercise CPU/RAM/NIC edits and disk expansion. Confirm unknown subfields and
   other NICs are preserved, pending restart state is accurate, and no hardware
   update silently restarts the guest. Verify shrink rejection and explicit
   power commands only on disposable guests.
6. Interrupt deployment after task submission and during readiness. Inspect task
   IDs/status and resume. Confirm one clone, retained completed phases, and
   correct outcome. Simulate an uncertain submission and confirm the CLI refuses
   a duplicate mutation. Test stale digests and VMID collisions.
7. Test an unreachable guest, wrong SSH identity, and host-key mismatch. Confirm
   there is no success result and the created VM remains identifiable.
8. Test failed/canceled template provisioning. Inspect retained build resources,
   local credential cleanup, and answer-media cleanup/residual reporting. No
   failed build should be presented as an accepted template.

Record PVE/package versions, OS/VirtIO ISO hashes, Windows image name,
Cloudbase-Init MSI URL/hash, client/tool versions, operation IDs, and outcomes.
Remove only the recorded lab resources using the Proxmox UI or an independently
reviewed administrator procedure. VM deletion is intentionally absent from this
CLI. Release acceptance requires all three guest matrices to pass.
