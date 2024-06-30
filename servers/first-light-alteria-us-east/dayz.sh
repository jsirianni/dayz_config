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
mod_gsc_assets="3154500253"
mod_alteria="2811630720"
mod_code_lock="1646187754"
mod_buildanywhere="1854626456"
mod_bbp="1710977250"

dayz() {
    sudo -u dayz 'bash' <<EOF
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
EOF
}

# symlinks links the mod and key directories. NOTE: Sometimes the key directory
# is capitalized.
symlinks() {
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_cf" "/opt/dayz/$mod_cf"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_vpp" "/opt/dayz/$mod_vpp"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_gsc_assets" "/opt/dayz/$mod_gsc_assets"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_alteria" "/opt/dayz/$mod_alteria"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_code_lock" "/opt/dayz/$mod_code_lock"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_buildanywhere" "/opt/dayz/$mod_buildanywhere"
    sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_bbp" "/opt/dayz/$mod_bbp"

    # CF and Online Tools share the same key
    eval sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_cf/keys/*" /opt/dayz/keys/
    eval sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_vpp/keys/*" /opt/dayz/keys/
    eval sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_gsc_assets/keys/*" /opt/dayz/keys/
    # Alteria mod does not have a key?
    # eval sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_alteria/keys/*" /opt/dayz/keys/
    eval sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_code_lock/Keys/*" /opt/dayz/keys/
    eval sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_buildanywhere/Keys/*" /opt/dayz/keys/
    eval sudo ln -sf "/opt/dayz/steamapps/workshop/content/221100/$mod_bbp/keys/*" /opt/dayz/keys/

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
    -mod="$mod_cf;$mod_alteria;$mod_gsc_assets;$mod_vpp;$mod_code_lock;$mod_buildanywhere;$mod_bbp;" \
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
