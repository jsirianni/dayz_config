#!/usr/bin/env bash

# Setup script targeting Debian 12 (Bookworm).
#
# Bootstraps a dayz server, does not manage the server.

set -ex

shell_user="dayz"
shell_home="/opt/dayz"

refresh_repo() {
    sudo apt-get update -qq > /dev/null
}

user() {
    if getent group "$shell_user" &>/dev/null; then
        echo "Group ${shell_user} already exists."
    else
        sudo groupadd "$shell_user"
    fi

    if id "$shell_user" &>/dev/null; then
        echo "User ${shell_user} already exists"
        return
    fi
    
    sudo useradd \
        --shell /bin/bash \
        "$shell_user" \
        -g "$shell_user" \
        --home-dir "$shell_home"
}

homedir() {
    sudo mkdir -p "$shell_home"
    sudo chown -R "$shell_user:$shell_user" "$shell_home"
}

steamcmd() {
    refresh_repo
    sudo apt-get install -qq -y \
        software-properties-common \
        lib32gcc-s1 \
        > /dev/null
    sudo apt-add-repository non-free -y > /dev/null
    sudo dpkg --add-architecture i386 > /dev/null
    refresh_repo

    echo steam steam/question select "I AGREE" | sudo debconf-set-selections
    echo steam steam/license note '' | sudo debconf-set-selections

    sudo apt-get install steamcmd -y > /dev/null
}

user
homedir
steamcmd
