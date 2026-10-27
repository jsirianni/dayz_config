#!/bin/sh
# Read by Configure-Notifier.ps1. MODE, PAYLOAD (base64), and EXPECTED_SHA precede it.
set -eu
unit=dzsa-notifier.service
directory=/etc/dzsa-notifier
config=$directory/config.yaml
binary=/usr/bin/dzsa-notifier
test "$(id -u)" -eq 0 || { echo 'Administrative access is required' >&2; exit 1; }
test -x "$binary" || { echo 'Install dzsa-notifier first' >&2; exit 1; }
systemctl cat "$unit" >/dev/null
# The file is authoritative: never silently let existing service overrides shadow it.
if systemctl show-environment | grep -q '^DZSA_NOTIFIER_'; then
    echo 'Remove DZSA_NOTIFIER_ manager environment overrides first' >&2
    exit 1
fi
environment_files=$(systemctl show "$unit" --property=EnvironmentFiles --value)
case "$environment_files" in
    ''|'/etc/dzsa-notifier/environment (ignore_errors=yes)') ;;
    *) echo 'Additional systemd EnvironmentFiles are incompatible with authoritative YAML' >&2; exit 1 ;;
esac
if systemctl show "$unit" --property=Environment --value | grep -q 'DZSA_NOTIFIER_'; then
    echo 'Remove DZSA_NOTIFIER_ systemd environment overrides before applying authoritative YAML' >&2
    exit 1
fi
if [ -f "$directory/environment" ] && grep -Eq '^[[:space:]]*(export[[:space:]]+)?DZSA_NOTIFIER_' "$directory/environment"; then
    echo 'Remove DZSA_NOTIFIER_ overrides from /etc/dzsa-notifier/environment first' >&2
    exit 1
fi
if [ "$MODE" = preview ]; then
    printf 'Read-only preview: apply config SHA256 %s to %s and enable/start the notifier; restart only if changed.\n' "$EXPECTED_SHA" "$config"
    systemctl show "$unit" --property=ActiveState --property=UnitFileState
    if [ -f "$config" ]; then sha256sum "$config"; fi
    exit 0
fi
# Serialize configuration transactions and package/config operations by this script.
exec 9>/run/lock/dzsa-notifier-config.lock
flock -n 9 || { echo 'Another configuration operation is active' >&2; exit 1; }
install -d -m 0755 "$directory"
stage=$(mktemp "$directory/.config.XXXXXXXX")
was_active=no
was_enabled=no
had_config=no
changed=no
committed=no
transaction_changed=no
backup=
systemctl is-active --quiet "$unit" && was_active=yes
systemctl is-enabled --quiet "$unit" && was_enabled=yes
cleanup() {
    status=$?
    trap - EXIT HUP INT TERM
    if [ "$committed" != yes ] && [ "$changed" = yes ]; then
        echo 'Activation failed; restoring prior configuration and service state' >&2
        if [ "$transaction_changed" = yes ]; then
            if [ "$had_config" = yes ]; then
                cp -p "$backup" "$stage"
                mv -f "$stage" "$config"
            else
                rm -f "$config"
            fi
        fi
        if [ "$was_enabled" = yes ]; then systemctl enable "$unit"; else systemctl disable "$unit"; fi
        if [ "$was_active" = yes ]; then systemctl restart "$unit"; else systemctl stop "$unit"; fi
    fi
    rm -f "$stage"
    exit "$status"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM
printf '%s' "$PAYLOAD" | base64 -d > "$stage"
printf '%s  %s\n' "$EXPECTED_SHA" "$stage" | sha256sum -c -
# Configuration validation performs no notification and may run administratively.
env -u DZSA_NOTIFIER_INTERVAL -u DZSA_NOTIFIER_REQUEST_TIMEOUT -u DZSA_NOTIFIER_METRICS_LISTEN -u DZSA_NOTIFIER_ENDPOINTS "$binary" --check-config --config "$stage"
if [ -f "$config" ]; then
    had_config=yes
    if cmp -s "$stage" "$config"; then
        changed=no
    else
        backup="$directory/config.yaml.bak.$(date -u +%Y%m%dT%H%M%SZ).$$"
        cp -p "$config" "$backup"
        changed=yes
    fi
else
    changed=yes
fi
# Also roll back enable/start failures when the YAML itself is unchanged.
transaction_changed=$changed
if [ "$was_active" != yes ] || [ "$was_enabled" != yes ]; then changed=yes; fi
if [ "$transaction_changed" = yes ]; then
    chmod 0644 "$stage"
    chown root:root "$stage"
    mv -f "$stage" "$config"
fi
systemctl enable "$unit"
if [ "$transaction_changed" = yes ] && [ "$was_active" = yes ]; then
    systemctl restart "$unit"
else
    systemctl start "$unit"
fi
systemctl is-active --quiet "$unit"
pid=$(systemctl show "$unit" --property=MainPID --value)
uid=$(ps -o uid= -p "$pid" | tr -d ' ')
test -n "$uid" && test "$uid" != 0
committed=yes
printf 'Notifier configuration active. PID=%s UID=%s configuration_changed=%s\n' "$pid" "$uid" "$transaction_changed"
if [ -n "$backup" ]; then printf 'Previous configuration: %s\n' "$backup"; fi
