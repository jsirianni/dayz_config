#!/bin/sh
# Run as root on a disposable Ubuntu runner with PowerShell and OpenSSH installed.
set -eu
package=$1
fixture=$2
repo=$(pwd)
work=$(mktemp -d /var/lib/dzsa-notifier-ssh.XXXXXXXX)
fixture_pid=
sshd_pid=
cleanup() {
    status=$?
    if [ "$status" -ne 0 ]; then tail -n 40 "$work/sshd.log" 2>/dev/null || true; fi
    systemctl stop dzsa-notifier.service 2>/dev/null || true
    if [ -n "$fixture_pid" ]; then kill "$fixture_pid" 2>/dev/null || true; fi
    if [ -n "$sshd_pid" ]; then kill "$sshd_pid" 2>/dev/null || true; fi
}
trap cleanup EXIT
dpkg -i "$package"
rm -f /etc/dzsa-notifier/config.yaml
chmod 0755 "$work"
"$fixture" --directory "$work" >"$work/fixture.log" 2>&1 &
fixture_pid=$!
for i in $(seq 1 50); do [ -f "$work/ca.pem" ] && break; sleep .1; done
install -d /etc/systemd/system/dzsa-notifier.service.d /run/sshd
cat >/etc/systemd/system/dzsa-notifier.service.d/test.conf <<EOF
[Service]
Environment=HTTPS_PROXY=http://127.0.0.1:18443
Environment=SSL_CERT_FILE=$work/ca.pem
EOF
systemctl daemon-reload
ssh-keygen -q -t ed25519 -N '' -f "$work/identity"
ssh-keygen -q -t ed25519 -N '' -f "$work/host"
ssh-keygen -q -t ed25519 -N '' -f "$work/other-host"
cp "$work/identity.pub" "$work/authorized_keys"
cat >"$work/sshd_config" <<EOF
Port 22222
ListenAddress 127.0.0.1
HostKey $work/host
AuthorizedKeysFile $work/authorized_keys
StrictModes no
PermitRootLogin prohibit-password
PasswordAuthentication no
KbdInteractiveAuthentication no
UsePAM yes
AllowUsers root ${SUDO_USER:-root}
PidFile $work/sshd.pid
EOF
/usr/sbin/sshd -D -e -f "$work/sshd_config" >"$work/sshd.log" 2>&1 &
sshd_pid=$!
sleep 1
write_config() {
    cat >"$work/config.yaml" <<EOF
interval: 1s
request_timeout: 2s
metrics_listen: 127.0.0.1:$1
endpoints: [$2]
EOF
}
apply() {
    pwsh -NoProfile -File "$repo/scripts/Configure-Notifier.ps1" -SSHHost 127.0.0.1 -User "${SSH_USER:-root}" -Port 22222 -IdentityFile "$work/identity" -KnownHostsFile "$work/known hosts" -ConfigPath "$work/config.yaml" "$@"
}
# Failed first activation must restore missing configuration and disabled service.
write_config 18443 '203.0.113.10:27016'
if apply; then echo 'Initial activation failure accepted'; exit 1; fi
test ! -e /etc/dzsa-notifier/config.yaml
if systemctl is-enabled --quiet dzsa-notifier.service; then echo 'Enablement not restored'; exit 1; fi
if systemctl is-active --quiet dzsa-notifier.service; then echo 'Running state not restored'; exit 1; fi
write_config 19091 '203.0.113.10:27016'
apply
# Exercise the prepared administrator + sudo path as well as direct root SSH.
if [ -n "${SUDO_USER:-}" ]; then SSH_USER=$SUDO_USER apply; fi
pid=$(systemctl show dzsa-notifier.service --property=MainPID --value)
before=$(sha256sum /etc/dzsa-notifier/config.yaml)
apply
test "$pid" = "$(systemctl show dzsa-notifier.service --property=MainPID --value)"
apply -WhatIf
test "$before" = "$(sha256sum /etc/dzsa-notifier/config.yaml)"
test "$pid" = "$(systemctl show dzsa-notifier.service --property=MainPID --value)"
write_config 19091 '203.0.113.10:27016, 203.0.113.10:27019'
apply
test "$pid" != "$(systemctl show dzsa-notifier.service --property=MainPID --value)"
grep -q 27019 /etc/dzsa-notifier/config.yaml
write_config 19091 '203.0.113.10:27019'
apply
if grep -q 27016 /etc/dzsa-notifier/config.yaml; then echo 'Removal not applied'; exit 1; fi
before=$(sha256sum /etc/dzsa-notifier/config.yaml)
pid=$(systemctl show dzsa-notifier.service --property=MainPID --value)
printf 'endpoints: [192.168.1.1:27016]\n' >"$work/config.yaml"
if apply; then echo 'Invalid config accepted'; exit 1; fi
test "$before" = "$(sha256sum /etc/dzsa-notifier/config.yaml)"
test "$pid" = "$(systemctl show dzsa-notifier.service --property=MainPID --value)"
# Occupied proxy port causes activation failure and must restore the previous YAML.
write_config 18443 '203.0.113.10:27016'
if apply; then echo 'Activation failure accepted'; exit 1; fi
test "$before" = "$(sha256sum /etc/dzsa-notifier/config.yaml)"
systemctl is-active --quiet dzsa-notifier.service
systemctl is-enabled --quiet dzsa-notifier.service
test -n "$(find /etc/dzsa-notifier -name 'config.yaml.bak.*' -print -quit)"
# A supplied changed known key must never be replaced, even during preview.
printf '[127.0.0.1]:22222 %s\n' "$(cat "$work/other-host.pub")" >"$work/known hosts"
write_config 19091 '203.0.113.10:27016'
if apply; then echo 'Changed host key accepted'; exit 1; fi
test "$before" = "$(sha256sum /etc/dzsa-notifier/config.yaml)"
echo 'SSH configuration, rollback, rerun, WhatIf, and host-key checks passed'
