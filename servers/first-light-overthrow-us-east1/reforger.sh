#!/usr/bin/env bash
# Idempotent Arma Reforger dedicated server setup for Debian
# - Requires root (or sudo)
# - Sources /opt/reforger.env for secrets (PASSWORD_ADMIN)
# - Adds user "reforger" (system user, no shell)
# - Creates directories and sets ownership
# - Installs dependency packages
# - Downloads/updates server via SteamCMD (anonymous)
# - Creates/updates systemd service (does NOT enable/start/restart)
# - Writes server.json from a heredoc "template" using env vars

# ---- Require root (works for direct root or sudo) ----
if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
  echo "This script must be run as root (or via sudo)." >&2
  exit 1
fi

set -euo pipefail

# ---- Load env (fail if missing or incomplete) ----
ENV_FILE="/opt/reforger.env"
if [[ ! -f "$ENV_FILE" ]]; then
  echo "Missing $ENV_FILE. Create it with: PASSWORD_ADMIN=your_admin_password" >&2
  exit 1
fi
# shellcheck disable=SC1090
source "$ENV_FILE"

: "${PASSWORD_ADMIN:?PASSWORD_ADMIN is required in $ENV_FILE}"

# ---- Settings ----
ARMA_USER="reforger"
ARMA_HOME="/opt/arma-reforger"
ARMA_DIR_SERVER="${ARMA_HOME}/server"
ARMA_DIR_CONFIGS="${ARMA_HOME}/configs"
ARMA_DIR_LOGS="${ARMA_HOME}/logs"
ARMA_PROFILE="${ARMA_HOME}/profile"
STEAM_APP_ID="1874900"   # Arma Reforger Dedicated Server
SYSTEMD_UNIT="/etc/systemd/system/arma-reforger.service"
EXEC_START_FLAGS="-config ${ARMA_DIR_CONFIGS}/server.json -loadSessionSave -maxFPS 120 -profile ${ARMA_PROFILE}"
APT_PACKAGES=(ca-certificates libstdc++6 lib32gcc-s1 tzdata)

# Overthrow scenario & mods
SCENARIO_ID="{3DAD390C31623F04}Missions/24_OVT_Eden.conf"
MODS_JSON='
    { "modId": "59B657D731E2A11D", "name": "Overthrow", "required": true },
    { "modId": "5D6EA74A94173EDF", "name": "Enfusion Database Framework", "required": true },
    { "modId": "5D6EBC81EB1842EF", "name": "Enfusion Persistence Framework", "required": true }
'

PUBLIC_ADDRESS="overthrow.reforger.first-light.coffeedns.net"
SERVER_NAME="[Overthrow] First Light US-EAST1"

# ---- Helpers ----
check_steamcmd() {
  if command -v steamcmd >/dev/null 2>&1; then
    STEAMCMD="steamcmd"
  elif [[ -x "/usr/games/steamcmd" ]]; then
    STEAMCMD="/usr/games/steamcmd"
  elif [[ -x "${ARMA_HOME}/steamcmd/steamcmd.sh" ]]; then
    STEAMCMD="${ARMA_HOME}/steamcmd/steamcmd.sh"
  else
    echo "steamcmd not found. Please install SteamCMD and re-run." >&2
    exit 1
  fi
}

ensure_user() {
  if id -u "${ARMA_USER}" >/dev/null 2>&1; then
    echo "User ${ARMA_USER} already exists."
  else
    echo "Creating system user ${ARMA_USER}..."
    if ! getent group "${ARMA_USER}" >/dev/null 2>&1; then
      groupadd --system "${ARMA_USER}"
    fi
    useradd --system \
      --home-dir "${ARMA_HOME}" \
      --create-home \
      --shell /usr/sbin/nologin \
      --gid "${ARMA_USER}" \
      "${ARMA_USER}"
  fi
}

ensure_dirs() {
  echo "Ensuring directories..."
  install -d -m 0755 -o "${ARMA_USER}" -g "${ARMA_USER}" "${ARMA_HOME}"
  install -d -m 0755 -o "${ARMA_USER}" -g "${ARMA_USER}" "${ARMA_DIR_SERVER}"
  install -d -m 0755 -o "${ARMA_USER}" -g "${ARMA_USER}" "${ARMA_DIR_CONFIGS}"
  install -d -m 0755 -o "${ARMA_USER}" -g "${ARMA_USER}" "${ARMA_DIR_LOGS}"
  install -d -m 0755 -o "${ARMA_USER}" -g "${ARMA_USER}" "${ARMA_PROFILE}"
  chown -R "${ARMA_USER}:${ARMA_USER}" "${ARMA_HOME}"
}

install_packages() {
  if command -v apt-get >/dev/null 2>&1; then
    echo "Installing dependency packages (if needed): ${APT_PACKAGES[*]}"
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -y
    apt-get install -y --no-install-recommends "${APT_PACKAGES[@]}"
    apt-get autoremove -y
    apt-get clean
  else
    echo "apt-get not found; skipping package installation."
  fi
}

download_or_update_server() {
  echo "Downloading/updating Arma Reforger server (AppID ${STEAM_APP_ID})..."
  sudo -u "${ARMA_USER}" bash -lc "
    \"${STEAMCMD}\" +login anonymous \
      +force_install_dir \"${ARMA_DIR_SERVER}\" \
      +app_update ${STEAM_APP_ID} validate \
      +quit
  "
}

write_systemd_unit() {
  echo "Writing systemd unit to ${SYSTEMD_UNIT}..."
  cat > "${SYSTEMD_UNIT}" <<UNIT
[Unit]
Description=Arma Reforger Dedicated Server
After=network-online.target
Wants=network-online.target

[Service]
User=${ARMA_USER}
Group=${ARMA_USER}
WorkingDirectory=${ARMA_DIR_SERVER}
ExecStart=${ARMA_DIR_SERVER}/ArmaReforgerServer ${EXEC_START_FLAGS}
Restart=on-failure
RestartSec=5
StandardOutput=append:${ARMA_DIR_LOGS}/server.log
StandardError=append:${ARMA_DIR_LOGS}/server.err
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
UNIT
  chmod 0644 "${SYSTEMD_UNIT}"
  echo "Reloading systemd daemon (not starting/enabling the service)..."
  systemctl daemon-reload
}

write_server_json() {
  echo "Writing ${ARMA_DIR_CONFIGS}/server.json ..."
  # Use an expanding heredoc to template env vars
  cat > "${ARMA_DIR_CONFIGS}/server.json" <<JSON
{
  "publicPort": 2001,
  "bindAddress": "0.0.0.0",
  "bindPort": 2001,

  "a2s": { "address": "0.0.0.0", "port": 17777 },

  "rcon": {
    "address": "0.0.0.0",
    "port": 19999,
    "password": "${PASSWORD_ADMIN}",
    "permission": "admin",
    "blacklist": [],
    "whitelist": []
  },

  "game": {
    "name": "${SERVER_NAME}",
    "password": "",
    "passwordAdmin": "${PASSWORD_ADMIN}",
    "admins": [],
    "scenarioId": "${SCENARIO_ID}",
    "maxPlayers": 64,
    "visible": true,
    "crossPlatform": true,
    "gameProperties": {
      "serverMaxViewDistance": 1600,
      "serverMinGrassDistance": 50,
      "networkViewDistance": 1500,
      "disableThirdPerson": false,
      "fastValidation": true,
      "battlEye": true,
      "VONDisableUI": false,
      "VONDisableDirectSpeechUI": false,
      "VONCanTransmitCrossFaction": true,
      "missionHeader": {}
    },
    "mods": [
${MODS_JSON}
    ],
    "modsRequiredByDefault": false
  },

  "operating": {
    "lobbyPlayerSynchronise": true,
    "playerSaveTime": 120,
    "aiLimit": -1,
    "slotReservationTimeout": 60,
    "disableServerShutdown": false,
    "disableCrashReporter": false,
    "disableAI": false,
    "joinQueue": { "maxSize": 0 }
  }
}
JSON
  chown "${ARMA_USER}:${ARMA_USER}" "${ARMA_DIR_CONFIGS}/server.json"
  chmod 0644 "${ARMA_DIR_CONFIGS}/server.json"
}

# ---- Main ----
check_steamcmd
ensure_user
ensure_dirs
install_packages
download_or_update_server
write_systemd_unit
write_server_json

echo "Done. Review ${ARMA_DIR_CONFIGS}/server.json and start with: systemctl start arma-reforger"
# As requested, we do NOT enable/start/restart the service.
