#!/usr/bin/env bash
set -euo pipefail

# Install Apache Kafka (KRaft mode) on Ubuntu.

SUDO=""
if [ "$(id -u)" -ne 0 ]; then
  SUDO="sudo"
fi

KAFKA_VERSION="${KAFKA_VERSION:-3.7.1}"
SCALA_VERSION="${SCALA_VERSION:-2.13}"
KAFKA_DIST="kafka_${SCALA_VERSION}-${KAFKA_VERSION}"
KAFKA_TGZ="${KAFKA_DIST}.tgz"
KAFKA_URL="https://downloads.apache.org/kafka/${KAFKA_VERSION}/${KAFKA_TGZ}"

log() {
  echo "[install-kafka] $*"
}

log "Updating apt index..."
$SUDO apt-get update -y

log "Installing dependencies..."
$SUDO apt-get install -y curl ca-certificates openjdk-17-jre-headless uuid-runtime

if ! id -u kafka >/dev/null 2>&1; then
  log "Creating kafka user..."
  $SUDO useradd --system --create-home --home-dir /var/lib/kafka --shell /usr/sbin/nologin kafka
fi

log "Downloading Kafka ${KAFKA_VERSION}..."
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT
curl -fSL "$KAFKA_URL" -o "${tmp_dir}/${KAFKA_TGZ}"

log "Installing Kafka to /opt/kafka..."
$SUDO mkdir -p /opt/kafka
$SUDO tar -xzf "${tmp_dir}/${KAFKA_TGZ}" -C /opt/kafka
$SUDO ln -sfn "/opt/kafka/${KAFKA_DIST}" /opt/kafka/current
$SUDO chown -R kafka:kafka "/opt/kafka/${KAFKA_DIST}"

log "Preparing Kafka data directory..."
$SUDO mkdir -p /var/lib/kafka
$SUDO chown -R kafka:kafka /var/lib/kafka

log "Writing Kafka KRaft configuration..."
$SUDO mkdir -p /etc/kafka/kraft
$SUDO tee /etc/kafka/kraft/server.properties >/dev/null <<'EOF'
process.roles=broker,controller
node.id=1
controller.quorum.voters=1@127.0.0.1:9093
listeners=PLAINTEXT://0.0.0.0:9092,CONTROLLER://0.0.0.0:9093
listener.security.protocol.map=PLAINTEXT:PLAINTEXT,CONTROLLER:PLAINTEXT
inter.broker.listener.name=PLAINTEXT
controller.listener.names=CONTROLLER
log.dirs=/var/lib/kafka
auto.create.topics.enable=false
EOF

log "Formatting Kafka storage..."
cluster_id="$($SUDO -u kafka /opt/kafka/current/bin/kafka-storage.sh random-uuid)"
$SUDO -u kafka /opt/kafka/current/bin/kafka-storage.sh format -t "$cluster_id" -c /etc/kafka/kraft/server.properties

log "Writing systemd unit..."
$SUDO tee /etc/systemd/system/kafka.service >/dev/null <<'EOF'
[Unit]
Description=Apache Kafka (KRaft)
After=network.target

[Service]
Type=simple
User=kafka
Group=kafka
ExecStart=/opt/kafka/current/bin/kafka-server-start.sh /etc/kafka/kraft/server.properties
ExecStop=/opt/kafka/current/bin/kafka-server-stop.sh
Restart=on-failure
RestartSec=5s
LimitNOFILE=100000

[Install]
WantedBy=multi-user.target
EOF

log "Enabling and starting kafka service..."
$SUDO systemctl daemon-reload
$SUDO systemctl enable --now kafka

log "Done."
log "Kafka service: systemctl status kafka"
