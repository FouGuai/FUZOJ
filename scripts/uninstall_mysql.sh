#!/usr/bin/env bash
set -euo pipefail

# Uninstall MySQL on Ubuntu.

SUDO=""
if [ "$(id -u)" -ne 0 ]; then
  SUDO="sudo"
fi

log() {
  echo "[uninstall-mysql] $*"
}

log "Stopping mysql service (ignore errors if not running)..."
$SUDO systemctl stop mysql || true

log "Removing mysql-server package..."
$SUDO apt-get remove -y mysql-server
$SUDO apt-get autoremove -y

log "Done."
log "Data and config are preserved at /var/lib/mysql and /etc/mysql."
log "Remove them manually if you want a full purge."
