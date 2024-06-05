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
mod_online_tools="1564026768"
mod_admin_tools="1582756848"
mod_code_lock="1646187754"
mod_anastara="2973953648"
mod_anastara_pack="3076836722"
mod_SimpleSpawner="2868802243"
mod_SpawnerBubaku="2482312670"
mod_Dabs="2545327648"
mod_WindstridesClothingPack="1797720064"

dayz() {
    sudo -u dayz 'bash' <<EOF
/usr/games/steamcmd \
    +force_install_dir "$shell_home" \
    +login "$STEAM_USER" \
    +app_update "$DAYZ_APP_ID" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mod_cf" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mod_online_tools" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mod_admin_tools" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mod_code_lock" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mod_anastara" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mod_anastara_pack" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mod_SimpleSpawner" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mod_SpawnerBubaku" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mod_Dabs" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mod_WindstridesClothingPack" \
    +quit
EOF
}

# symlinks links the mod and key directories. NOTE: Sometimes the key directory
# is capitalized.
symlinks() {
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_cf" "/opt/dayz/$mod_cf"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_online_tools" "/opt/dayz/$mod_online_tools"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_admin_tools" "/opt/dayz/$mod_admin_tools"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_code_lock" "/opt/dayz/$mod_code_lock"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_anastara" "/opt/dayz/$mod_anastara"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_anastara_pack" "/opt/dayz/$mod_anastara_pack"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_SimpleSpawner" "/opt/dayz/$mod_SimpleSpawner"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_SpawnerBubaku" "/opt/dayz/$mod_SpawnerBubaku"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_Dabs" "/opt/dayz/$mod_Dabs"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_WindstridesClothingPack" "/opt/dayz/$mod_WindstridesClothingPack"

    # CF and Online Tools share the same key
    eval sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_cf/keys/*" /opt/dayz/keys/
    eval sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_admin_tools/Keys/*" /opt/dayz/keys/
    eval sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_code_lock/Keys/*" /opt/dayz/keys/
    eval sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_anastara/Keys/*" /opt/dayz/keys/
    eval sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_anastara_pack/Keys/*" /opt/dayz/keys/
    eval sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_SimpleSpawner/Keys/*" /opt/dayz/keys/
    eval sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_SpawnerBubaku/Keys/*" /opt/dayz/keys/
    eval sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_Dabs/Keys/*" /opt/dayz/keys/
    eval sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_WindstridesClothingPack/Keys/*" /opt/dayz/keys/

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
    -port=2301 \
    -mod="$mod_cf;$mod_online_tools;$mod_admin_tools;$mod_code_lock;$mod_anastara;$mod_anastara_pack;$mod_SimpleSpawner;$mod_SpawnerBubaku;$mod_Dabs;$mod_WindstridesClothingPack;" \
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
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
}

dayz
symlinks
systemd
