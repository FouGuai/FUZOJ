#!/usr/bin/env bash
set -euo pipefail

# Install MySQL on Ubuntu.

SUDO=""
if [ "$(id -u)" -ne 0 ]; then
  SUDO="sudo"
fi

log() {
  echo "[install-mysql] $*"
}

log "Updating apt index..."
$SUDO apt-get update -y

log "Installing mysql-server..."
$SUDO apt-get install -y mysql-server

log "Enabling and starting mysql service..."
$SUDO systemctl enable --now mysql

log "Done."
log "MySQL service: systemctl status mysql"
