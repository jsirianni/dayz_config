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

shell_home="/opt/dayz"

mod_cf="1559212036"
mod_vpp="1828439124"
mod_trader="1590841260"
mod_map="1623711988"
mod_party="1582671564"
mode_boats="3354681846"
thirdpp="2122332595"
cars="3369325490"
gunz="2443122116"
mil_gear="1630943713"

dayz() {
    sudo -u dayz 'bash' <<EOF
/usr/games/steamcmd \
    +force_install_dir "$shell_home" \
    +login "$STEAM_USER" \
    +app_update "$DAYZ_APP_ID" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mod_cf" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mod_vpp" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mod_trader" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mod_map" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mod_party" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mode_boats" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$thirdpp" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$cars" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$gunz" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mil_gear" \
    +quit
EOF
}

# symlinks links the mod and key directories. NOTE: Sometimes the key directory
# is capitalized.
symlinks() {
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_cf" "/opt/dayz/$mod_cf"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_vpp" "/opt/dayz/$mod_vpp"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_trader" "/opt/dayz/$mod_trader"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_map" "/opt/dayz/$mod_map"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_party" "/opt/dayz/$mod_party"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mode_boats" "/opt/dayz/$mode_boats"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$thirdpp" "/opt/dayz/$thirdpp"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$cars" "/opt/dayz/$cars"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$gunz" "/opt/dayz/$gunz"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mil_gear" "/opt/dayz/$mil_gear"

    # Function to find and link keys directory for a mod
    link_mod_keys() {
        local mod_id="$1"
        local mod_path="/opt/dayz/steamapps/workshop/content/221100/$mod_id"
        
        # Check for different key directory variations
        for key_dir in "keys" "Keys" "key" "Key"; do
            if [ -d "$mod_path/$key_dir" ]; then
                echo "Found $key_dir directory for mod $mod_id"
                eval sudo ln -sf "$mod_path/$key_dir/*" /opt/dayz/keys/
                return 0
            fi
        done
        
        echo "Warning: No key directory found for mod $mod_id"
        return 1
    }

    # Link keys for each mod
    link_mod_keys "$mod_cf"
    link_mod_keys "$mod_vpp"
    link_mod_keys "$mod_trader"
    link_mod_keys "$mod_map"
    link_mod_keys "$mod_party"
    link_mod_keys "$mode_boats"
    link_mod_keys "$thirdpp"
    link_mod_keys "$cars"
    link_mod_keys "$gunz"
    link_mod_keys "$mil_gear"

    sudo chown -R dayz:dayz /opt/dayz
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
    -port=2501 \
    -mod="$mod_cf;$mod_vpp;$mod_trader;$mod_map;$mod_party;$mode_boats;$thirdpp;$cars;$gunz;$mil_gear;" \
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
systemd
