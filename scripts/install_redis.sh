#!/usr/bin/env bash
set -euo pipefail

# Install Redis on Ubuntu.

SUDO=""
if [ "$(id -u)" -ne 0 ]; then
  SUDO="sudo"
fi

log() {
  echo "[install-redis] $*"
}

log "Updating apt index..."
$SUDO apt-get update -y

log "Installing redis-server..."
$SUDO apt-get install -y redis-server

log "Enabling and starting redis-server service..."
$SUDO systemctl enable --now redis-server

log "Done."
log "Redis service: systemctl status redis-server"
