#!/usr/bin/env bash

set -e

# Pass steam username as argument
if [ -z "$1" ]; then
    echo "Usage: $0 <steam_username>"
    exit 1
fi

if [ "$(id -u)" -ne 0 ]; then
    # check if user has sudo privileges
    sudo -v
    if [ $? -ne 0 ]; then
        echo "Please run as root or with sudo"
        exit 1
    fi
fi

STEAM_USER="$1"
DAYZ_APP_ID=223350
DAYZ_WORKSHOP_ID=221100

mod_cf=1559212036
mod_vpp=1828439124

shell_home="/opt/dayz"

dayz() {
    sudo -u dayz 'bash' <<EOF
/usr/games/steamcmd \
    +force_install_dir "$shell_home" \
    +login "$STEAM_USER" \
    +app_update "$DAYZ_APP_ID" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mod_cf" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mod_vpp" \
    +quit
EOF
}

# Link Workshop mods and signing keys; accept either key-directory capitalization.
symlinks() {
    local workshop_root="$shell_home/steamapps/workshop/content/$DAYZ_WORKSHOP_ID"
    local mod_id key_file
    local -a key_files
    sudo mkdir -p "$shell_home/keys"
    shopt -s nullglob
    for mod_id in "$mod_cf" "$mod_vpp"; do
        if [ ! -d "$workshop_root/$mod_id" ]; then
            echo "Missing Workshop mod: $mod_id" >&2
            return 1
        fi
        key_files=("$workshop_root/$mod_id"/{keys,Keys}/*.bikey)
        if [ "${#key_files[@]}" -eq 0 ]; then
            echo "Missing signing keys for Workshop mod: $mod_id" >&2
            return 1
        fi
        sudo ln -sfnT "$workshop_root/$mod_id" "$shell_home/$mod_id"
        for key_file in "${key_files[@]}"; do
            sudo ln -sf "$key_file" "$shell_home/keys/"
        done
    done
    shopt -u nullglob
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
    -port=2301 \
    -mod="$mod_cf;$mod_vpp" \
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

sudo systemctl daemon-reload
}

dayz
symlinks
sudo chown -R dayz:dayz /opt/dayz
systemd
