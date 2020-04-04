#!/bin/bash

# IMPORTANT: make sure your editor is using 'LF' for new lines and not
# 'CRLF' as that will cayse cygwin to fail to run this script

# IMPORTANT: this script expects to run from your dayz server install directory

set -e

# steam values
STEAM_USER=$1
STEAM_DAYZ_SERVER_ID="223350"
STEAM_DAYZ_MOD_ID="221100"

# the directory this repo should live in
WIN_CONF_BASE="C:/dayz_config"
CYG_CONF_BASE="/cygdrive/c/dayz_config"

# workshop directory
WIN_WORKSHOP_BASE="C:/dayz_config/steamcmd/steamapps/workshop/content/${STEAM_DAYZ_MOD_ID}"
CYG_WORKSHOP_BASE="/cygdrive/c/dayz_config/steamcmd/steamapps/workshop/content/${STEAM_DAYZ_MOD_ID}"

# dayz server directory
WIN_SERVER_BASE="C:/steam/steamapps/common/dayz"
CYG_SERVER_BASE="/cygdrive/c/steam/steamapps/common/dayz"

# mod keys
CYG_SERVER_KEYS="/cygdrive/c/steam/steamapps/common/dayz/keys"

# program and file names
MOD_SERVER_BIN="DZSALModServer.exe"
DAYZ_CONF_NAME="serverDZ.cfg"
DAYZ_BAN_FILE="ban.txt"
DAYZ_WHITELIST="whitelist.txt"

# program paths
STEAM_CMD_BIN="${WIN_CONF_BASE}/steamcmd/steamcmd.exe"

# config files
DAYZ_MOD_CSV="${WIN_CONF_BASE}/mods.csv"
DAYZ_SERVER_CONF="${WIN_CONF_BASE}/"

# server config
DAYZ_SERVER_NAME="Hillary's Emails"
DAYZ_SERVER_PORT="2302"
DAYZ_SERVER_CPU="6"
DAYZ_MOD_LIST=""  # leave empty, this will be set by set_mod_list()
DAYZ_SERVER_OPTS="-dologs -adminlog -netlog -freezecheck"


update_dayz() {
  $STEAM_CMD_BIN +login $STEAM_USER \
    +force_install_dir $WIN_SERVER_BASE \
    +app_update $STEAM_DAYZ_SERVER_ID \
    validate +quit
}

update_mods () {
  while IFS=, read -r id name
  do
    # trim carriage return from string
    id=$(echo $id | tr -d '\r')
    name=$(echo $name | tr -d '\r')

    $STEAM_CMD_BIN +login $STEAM_USER \
      +workshop_download_item $STEAM_DAYZ_MOD_ID $id \
      +force_install_dir "${WIN_WORKSHOP_BASE}"/"${id}" \
      validate \
      +quit

    # do not modify anything here, as you can break how directory naming is handled
    # with spaces!
    rsync -r --progress ${CYG_WORKSHOP_BASE}/${id}/ "${CYG_SERVER_BASE}/${name}"
    rsync --progress "${CYG_WORKSHOP_BASE}"/"${id}"/Keys/* "${CYG_SERVER_KEYS}/"
  done < $DAYZ_MOD_CSV
}

deploy_mod_server () {
  cp -f ${CYG_CONF_BASE}/${MOD_SERVER_BIN} ${CYG_SERVER_BASE}/${MOD_SERVER_BIN}
}

deploy_config () {
  cp -f ${CYG_CONF_BASE}/${DAYZ_CONF_NAME} ${CYG_SERVER_BASE}/${DAYZ_CONF_NAME}
}

deploy_ban () {
  cp -f ${CYG_CONF_BASE}/${DAYZ_BAN_FILE} ${CYG_SERVER_BASE}/${DAYZ_BAN_FILE}
}

deploy_whitelist () {
  cp -f ${CYG_CONF_BASE}/${DAYZ_WHITELIST} ${CYG_SERVER_BASE}/${DAYZ_WHITELIST}
}

deploy_init_c () {
  cp -f init.c_chernarusplus ${CYG_SERVER_BASE}/mpmissions/dayzOffline.chernarusplus/init.c
}

set_mod_list () {
  while IFS=, read -r id name
  do
    # trim carriage return from string
    id=$(echo $id | tr -d '\r')
    name=$(echo $name | tr -d '\r')

    # handle mod names with spaces in the name, because windows people
    # love their spaces
    if [[ "$name" =~ \ |\' ]]
    then
      echo "detected space in mod name: ${name}, adding quotes to name"
      name="\"${name}\""
    fi

    DAYZ_MOD_LIST="${DAYZ_MOD_LIST}${name};"
  done < $DAYZ_MOD_CSV
}

run_dayz_server () {
  cd $CYG_SERVER_BASE
  cygstart $MOD_SERVER_BIN \
    -config=$DAYZ_CONF_NAME \
    -profiles=DayzProfiles \
    -port=$DAYZ_SERVER_PORT \
    -cpuCount=$DAYZ_SERVER_CPU \
    $DAYZ_SERVER_OPTS \
    "-mod=${DAYZ_MOD_LIST}" \
    "dayzserver=DayZServer_64.exe"
}

main () {
  update_dayz
  update_mods
  deploy_mod_server
  deploy_config
  deploy_ban
  deploy_whitelist
  deploy_init_c
  set_mod_list && echo "using modes: ${DAYZ_MOD_LIST}"
  run_dayz_server
}

main
