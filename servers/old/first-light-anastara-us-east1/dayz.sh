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
mod_anastara="2973953648"
mod_anastarta_pack="3076836722"
dabs_framework="2545327648"
windstrides="1797720064"

dayz() {
    sudo -u dayz 'bash' <<EOF
/usr/games/steamcmd \
    +force_install_dir "$shell_home" \
    +login "$STEAM_USER" \
    +app_update "$DAYZ_APP_ID" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mod_cf" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mod_vpp" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mod_anastara" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$mod_anastarta_pack" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$dabs_framework" \
    +workshop_download_item "$DAYZ_WORKSHOP_ID" "$windstrides" \
    +quit
EOF
}

# symlinks links the mod and key directories. NOTE: Sometimes the key directory
# is capitalized.
symlinks() {
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_cf" "/opt/dayz/$mod_cf"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_vpp" "/opt/dayz/$mod_vpp"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_anastara" "/opt/dayz/$mod_anastara"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_anastarta_pack" "/opt/dayz/$mod_anastarta_pack"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$dabs_framework" "/opt/dayz/$dabs_framework"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$windstrides" "/opt/dayz/$windstrides"

    eval sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_cf/keys/*" /opt/dayz/keys/
    eval sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_vpp/keys/*" /opt/dayz/keys/
    eval sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_anastara/keys/*" /opt/dayz/keys/
    eval sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_anastarta_pack/keys/*" /opt/dayz/keys/
    eval sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$dabs_framework/keys/*" /opt/dayz/keys/
    eval sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$windstrides/keys/*" /opt/dayz/keys/

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
    -port=3101 \
    -mod="$mod_cf;$mod_vpp;$mod_anastara;$mod_anastarta_pack;$dabs_framework;$windstrides" \
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
