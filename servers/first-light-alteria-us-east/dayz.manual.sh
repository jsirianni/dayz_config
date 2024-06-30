#!/usr/bin/env bash

set -e

# Make sure user is "dayz"
if [ "$(whoami)" != "dayz" ]; then
    echo "Please run as dayz user"
    exit 1
fi

# Pass steam username as argument
if [ -z "$1" ]; then
    echo "Usage: $0 <steam_username>"
    exit 1
fi

STEAM_USER="$1"
DAYZ_APP_ID=223350
DAYZ_WORKSHOP_ID=221100

shell_home="/opt/dayz"

mod_cf="1559212036"
mod_vpp="1828439124"
mod_gsc_assets="3154500253"
mod_alteria="2811630720"
mod_code_lock="1646187754"
mod_buildanywhere="1854626456"
mod_bbp="1710977250"

dayz() {
/usr/games/steamcmd \
    +force_install_dir "$shell_home" \
    +login "$STEAM_USER" \
    +app_update "$DAYZ_APP_ID" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mod_cf" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mod_vpp" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mod_gsc_assets" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mod_alteria" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mod_code_lock" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mod_buildanywhere" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mod_bbp" \
    +quit
}

systemd() {
    sudo tee /etc/systemd/system/dayz.service > /dev/null <<EOF
[Unit]
Description=DayZ Dedicated Server
Wants=network-online.target
After=syslog.target network.target nss-lookup.target network-online.target

[Service]
#ExecStartPre=/opt/update.sh
ExecStart=/opt/dayz/DayZServer \
    -config=serverDZ.cfg \
    -port=2601 \
    -BEpath=battleye \
    -profiles=profiles \
    -dologs \
    -adminlog \
    -netlog \
    -freezecheck
WorkingDirectory=/opt/dayz
LimitNOFILE=100000
ExecReload=/bin/kill -s HUP \$MAINPID
ExecStop=/bin/kill -s INT \$MAINPID
User=dayz
Group=dayz
Restart=always
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF
}

dayz
systemd
