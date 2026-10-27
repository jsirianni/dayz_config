#!/bin/sh
# Disposable systemd host only. No requests leave the local HTTPS fixture.
set -eu
format=$1
package=$2
fixture=$3
work=$(mktemp -d /var/lib/dzsa-notifier-test.XXXXXXXX)
fixture_pid=
cleanup() {
    systemctl stop dzsa-notifier.service 2>/dev/null || true
    if [ -n "$fixture_pid" ]; then kill "$fixture_pid" 2>/dev/null || true; fi
}
trap cleanup EXIT
"$fixture" --directory "$work" >"$work/fixture.log" 2>&1 &
fixture_pid=$!
chmod 0755 "$work"
for i in $(seq 1 50); do [ -f "$work/ca.pem" ] && break; sleep .1; done
install_package() {
    if [ "$format" = deb ]; then dpkg -i "$package"; else rpm -Uvh --replacepkgs "$package"; fi
}
install_package
if systemctl is-enabled --quiet dzsa-notifier.service; then echo 'Unexpected auto-enable'; exit 1; fi
if systemctl is-active --quiet dzsa-notifier.service; then echo 'Unexpected auto-start'; exit 1; fi
test -f /etc/dzsa-notifier/config.example.yaml
test ! -e /etc/dzsa-notifier/config.yaml
install -d /etc/systemd/system/dzsa-notifier.service.d
cat > /etc/systemd/system/dzsa-notifier.service.d/test.conf <<EOF
[Service]
Environment=HTTPS_PROXY=http://127.0.0.1:18443
Environment=SSL_CERT_FILE=$work/ca.pem
EOF
cat >/etc/dzsa-notifier/config.yaml <<EOF
interval: 1s
request_timeout: 2s
metrics_listen: 127.0.0.1:19090
endpoints: [203.0.113.10:27016, 203.0.113.10:27019]
EOF
chmod 0644 /etc/dzsa-notifier/config.yaml
systemctl daemon-reload
systemctl enable --now dzsa-notifier.service
pid=$(systemctl show dzsa-notifier.service --property=MainPID --value)
test "$(ps -o uid= -p "$pid" | tr -d ' ')" != 0
test "$(grep '^CapEff:' /proc/"$pid"/status | awk '{print $2}')" = 0000000000000000
for i in $(seq 1 50); do
    curl -fsS http://127.0.0.1:19090/metrics >"$work/metrics"
    grep -q 'dzsa_notifier_last_attempt_success{endpoint="203.0.113.10:27019"} 1' "$work/metrics" && break
    sleep .2
done
grep -q 'dzsa_notifier_last_attempt_success{endpoint="203.0.113.10:27019"} 1' "$work/metrics"
sha=$(sha256sum /etc/dzsa-notifier/config.yaml)
install_package
test "$sha" = "$(sha256sum /etc/dzsa-notifier/config.yaml)"
systemctl is-enabled --quiet dzsa-notifier.service
systemctl is-active --quiet dzsa-notifier.service
test "$pid" != "$(systemctl show dzsa-notifier.service --property=MainPID --value)"
# Root execution is rejected, but offline config validation is allowed.
set +e
/usr/bin/dzsa-notifier --once --config /etc/dzsa-notifier/config.yaml
status=$?
set -e
test "$status" -eq 77
systemctl stop dzsa-notifier.service
test "$(systemctl show dzsa-notifier.service --property=MainPID --value)" = 0
if [ "$format" = deb ]; then dpkg -r dzsa-notifier; else rpm -e dzsa-notifier; fi
test -f /etc/dzsa-notifier/config.yaml
test ! -e /etc/systemd/system/multi-user.target.wants/dzsa-notifier.service
echo "$format package lifecycle passed"
