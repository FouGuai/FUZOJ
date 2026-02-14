#!/usr/bin/env bash
set -euo pipefail

# Uninstall Apache Kafka on Ubuntu.

SUDO=""
if [ "$(id -u)" -ne 0 ]; then
  SUDO="sudo"
fi

log() {
  echo "[uninstall-kafka] $*"
}

log "Stopping kafka service (ignore errors if not running)..."
$SUDO systemctl stop kafka || true
$SUDO systemctl disable kafka || true

log "Removing systemd unit..."
$SUDO rm -f /etc/systemd/system/kafka.service
$SUDO systemctl daemon-reload

log "Removing Kafka binaries..."
$SUDO rm -rf /opt/kafka

log "Done."
log "Data and config are preserved at /var/lib/kafka and /etc/kafka."
log "Remove them manually if you want a full purge."
