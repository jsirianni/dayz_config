#!/bin/sh
set -eu
# Do not enable/start on first installation. Preserve a previously active service.
if [ -d /run/systemd/system ]; then
    systemctl daemon-reload
    if systemctl is-active --quiet dzsa-notifier.service; then
        systemctl restart dzsa-notifier.service
    fi
fi
