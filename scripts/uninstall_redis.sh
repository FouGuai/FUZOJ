#!/usr/bin/env bash
set -euo pipefail

# Uninstall Redis on Ubuntu.

SUDO=""
if [ "$(id -u)" -ne 0 ]; then
  SUDO="sudo"
fi

log() {
  echo "[uninstall-redis] $*"
}

log "Stopping redis-server service (ignore errors if not running)..."
$SUDO systemctl stop redis-server || true

log "Removing redis-server package..."
$SUDO apt-get remove -y redis-server
$SUDO apt-get autoremove -y

log "Done."
log "Data and config are preserved at /var/lib/redis and /etc/redis."
log "Remove them manually if you want a full purge."
