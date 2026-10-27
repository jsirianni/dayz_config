#!/bin/sh
set -eu
# DEB uses remove/deconfigure, RPM uses 0 for erase and 1 for upgrade.
case "${1:-}" in
    remove|deconfigure|0)
        if [ -d /run/systemd/system ]; then
            systemctl stop dzsa-notifier.service
            systemctl disable dzsa-notifier.service
        fi
        ;;
esac
