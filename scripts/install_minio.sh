#!/usr/bin/env bash
set -euo pipefail

# Install MinIO on Ubuntu.

SUDO=""
if [ "$(id -u)" -ne 0 ]; then
  SUDO="sudo"
fi

log() {
  echo "[install-minio] $*"
}

log "Updating apt index..."
$SUDO apt-get update -y

log "Installing dependencies..."
$SUDO apt-get install -y curl ca-certificates

if ! id -u minio >/dev/null 2>&1; then
  log "Creating minio user..."
  $SUDO useradd --system --create-home --home-dir /var/lib/minio --shell /usr/sbin/nologin minio
fi

log "Downloading MinIO server binary..."
$SUDO curl -fSL "https://dl.min.io/server/minio/release/linux-amd64/minio" -o /usr/local/bin/minio
$SUDO chmod +x /usr/local/bin/minio

log "Preparing MinIO data directory..."
$SUDO mkdir -p /var/lib/minio
$SUDO chown -R minio:minio /var/lib/minio

log "Writing MinIO config file..."
$SUDO mkdir -p /etc/minio
$SUDO tee /etc/minio/minio.conf >/dev/null <<'EOF'
# MinIO configuration file
MINIO_ROOT_USER=minioadmin
MINIO_ROOT_PASSWORD=minioadmin
MINIO_VOLUMES=/var/lib/minio
MINIO_OPTS="--console-address :9001"
EOF
$SUDO chmod 600 /etc/minio/minio.conf
$SUDO chown minio:minio /etc/minio/minio.conf

log "Writing systemd unit..."
$SUDO tee /etc/systemd/system/minio.service >/dev/null <<'EOF'
[Unit]
Description=MinIO Object Storage
After=network.target

[Service]
Type=simple
User=minio
Group=minio
EnvironmentFile=/etc/minio/minio.conf
ExecStart=/usr/local/bin/minio server ${MINIO_VOLUMES} ${MINIO_OPTS}
Restart=on-failure
RestartSec=5s
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF

log "Enabling and starting minio service..."
$SUDO systemctl daemon-reload
$SUDO systemctl enable --now minio

log "Done."
log "MinIO service: systemctl status minio"
log "Console: http://127.0.0.1:9001"
