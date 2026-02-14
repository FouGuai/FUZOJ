#!/usr/bin/env bash
set -euo pipefail

# Uninstall MinIO on Ubuntu.

SUDO=""
if [ "$(id -u)" -ne 0 ]; then
  SUDO="sudo"
fi

log() {
  echo "[uninstall-minio] $*"
}

log "Stopping minio service (ignore errors if not running)..."
$SUDO systemctl stop minio || true
$SUDO systemctl disable minio || true

log "Removing systemd unit..."
$SUDO rm -f /etc/systemd/system/minio.service
$SUDO systemctl daemon-reload

log "Removing MinIO binary..."
$SUDO rm -f /usr/local/bin/minio

log "Done."
log "Data and config are preserved at /var/lib/minio and /etc/minio."
log "Remove them manually if you want a full purge."
