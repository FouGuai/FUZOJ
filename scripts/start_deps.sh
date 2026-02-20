#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/deploy/docker-compose.deps.yml"
PROFILE_PATH="$ROOT_DIR/configs/dev.generated/dev-profile.runtime.yaml"
OUTPUT_DIR="$ROOT_DIR/configs/dev.generated"
COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-fuzoj}"

log() {
  echo "[deps] $*"
}

step() {
  local title="$1"
  shift
  log "$title"
  "$@"
}

cleanup_terminal() {
  if [[ -t 1 ]]; then
    if command -v tput >/dev/null 2>&1; then
      tput cnorm >/dev/null 2>&1 || true
    fi
    stty sane >/dev/null 2>&1 || true
    printf '\n'
  fi
}

trap cleanup_terminal EXIT INT TERM

require_command() {
  local name="$1"
  if ! command -v "$name" >/dev/null 2>&1; then
    echo "missing required tool: $name" >&2
    exit 1
  fi
}

pick_free_ports() {
  local count="$1"
  python3 - "$count" <<'PY'
import socket
import sys

count = int(sys.argv[1])
ports = []
for _ in range(count):
    s = socket.socket()
    s.bind(("127.0.0.1", 0))
    ports.append(s.getsockname()[1])
    s.close()
print(" ".join(str(p) for p in ports))
PY
}

compose() {
  COMPOSE_PROJECT_NAME="$COMPOSE_PROJECT_NAME" docker compose --ansi=never -f "$COMPOSE_FILE" "$@"
}

wait_for_mysql() {
  local max_attempts=180
  local attempt=1
  while [[ $attempt -le $max_attempts ]]; do
    if compose exec -T mysql mysqladmin ping -h 127.0.0.1 -uroot -proot >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
    attempt=$((attempt + 1))
  done
  echo "mysql did not become ready in time" >&2
  exit 1
}

wait_for_redis() {
  local max_attempts=60
  local attempt=1
  while [[ $attempt -le $max_attempts ]]; do
    if compose exec -T redis redis-cli ping >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
    attempt=$((attempt + 1))
  done
  echo "redis did not become ready in time" >&2
  exit 1
}

wait_for_kafka() {
  local max_attempts=60
  local attempt=1
  while [[ $attempt -le $max_attempts ]]; do
    if compose exec -T kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server 127.0.0.1:9092 --list >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
    attempt=$((attempt + 1))
  done
  echo "kafka did not become ready in time" >&2
  exit 1
}

wait_for_minio() {
  local network="${COMPOSE_PROJECT_NAME}_default"
  local max_attempts=60
  local attempt=1
  while [[ $attempt -le $max_attempts ]]; do
    if docker run --rm --network "$network" \
      -e "MC_HOST_local=http://minioadmin:minioadmin@minio:9000" \
      minio/mc ls local >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
    attempt=$((attempt + 1))
  done
  echo "minio did not become ready in time" >&2
  exit 1
}

wait_for_elasticsearch() {
  local network="${COMPOSE_PROJECT_NAME}_default"
  local max_attempts=90
  local attempt=1
  while [[ $attempt -le $max_attempts ]]; do
    if docker run --rm --network "$network" curlimages/curl:8.6.0 \
      -fsS http://elasticsearch:9200/_cluster/health >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
    attempt=$((attempt + 1))
  done
  echo "elasticsearch did not become ready in time" >&2
  exit 1
}

wait_for_kibana() {
  local network="${COMPOSE_PROJECT_NAME}_default"
  local max_attempts=120
  local attempt=1
  while [[ $attempt -le $max_attempts ]]; do
    if docker run --rm --network "$network" curlimages/curl:8.6.0 \
      -fsS http://kibana:5601/api/status >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
    attempt=$((attempt + 1))
  done
  echo "kibana did not become ready in time" >&2
  exit 1
}

init_mysql() {
  compose exec -T mysql mysql -h 127.0.0.1 -uroot -proot <<'SQL'
CREATE DATABASE IF NOT EXISTS fuzoj CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
CREATE USER IF NOT EXISTS 'user'@'%' IDENTIFIED BY 'password';
GRANT ALL PRIVILEGES ON fuzoj.* TO 'user'@'%';
FLUSH PRIVILEGES;
SQL
}

import_schema() {
  local schema_path="$1"
  if [[ ! -f "$schema_path" ]]; then
    echo "schema file not found: $schema_path" >&2
    exit 1
  fi
  compose exec -T mysql mysql -uroot -proot fuzoj <"$schema_path"
}

ensure_minio_buckets() {
  local network="${COMPOSE_PROJECT_NAME}_default"
  local max_attempts=30
  local attempt=1
  local timeout_cmd=()
  if command -v timeout >/dev/null 2>&1; then
    timeout_cmd=(timeout 60s)
  fi
  while [[ $attempt -le $max_attempts ]]; do
    if "${timeout_cmd[@]}" docker run --rm --network "$network" \
      -e "MC_HOST_local=http://minioadmin:minioadmin@minio:9000" \
      minio/mc mb --ignore-existing local/problem-data >/dev/null 2>&1 \
      && "${timeout_cmd[@]}" docker run --rm --network "$network" \
      -e "MC_HOST_local=http://minioadmin:minioadmin@minio:9000" \
      minio/mc mb --ignore-existing local/fuzoj >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
    attempt=$((attempt + 1))
  done
  echo "minio bucket initialization failed; last error output:" >&2
  "${timeout_cmd[@]}" docker run --rm --network "$network" \
    -e "MC_HOST_local=http://minioadmin:minioadmin@minio:9000" \
    minio/mc mb --ignore-existing local/problem-data 2>&1 | tail -n 5 >&2 || true
  "${timeout_cmd[@]}" docker run --rm --network "$network" \
    -e "MC_HOST_local=http://minioadmin:minioadmin@minio:9000" \
    minio/mc mb --ignore-existing local/fuzoj 2>&1 | tail -n 5 >&2 || true
  exit 1
}

ensure_kafka_topics() {
  local topics
  topics="$($ROOT_DIR/scripts/ensure_kafka_topics.py)"
  if [[ -z "$topics" ]]; then
    echo "no kafka topics found in configs" >&2
    return 1
  fi
  python3 - <<'PY' "$ROOT_DIR" "$topics"
import sys
from pathlib import Path

try:
    import yaml
except ImportError as exc:
    raise SystemExit("PyYAML is required to parse configs; please install pyyaml") from exc

root = Path(sys.argv[1])
topics = set(sys.argv[2].split())
cfg_path = root / "configs" / "submit_service.yaml"
cfg = yaml.safe_load(cfg_path.read_text(encoding="utf-8")) or {}

expected = set()
direct_topics = cfg.get("topics", {})
if isinstance(direct_topics, dict):
    for value in direct_topics.values():
        if isinstance(value, str) and value:
            expected.add(value)

submit_cfg = cfg.get("submit", {})
if isinstance(submit_cfg, dict):
    value = submit_cfg.get("statusFinalTopic")
    if isinstance(value, str) and value:
        expected.add(value)
    consumer = submit_cfg.get("statusFinalConsumer", {})
    if isinstance(consumer, dict):
        value = consumer.get("deadLetterTopic")
        if isinstance(value, str) and value:
            expected.add(value)

missing = sorted(expected - topics)
if missing:
    print("kafka topics missing from init list: " + ", ".join(missing), file=sys.stderr)
    raise SystemExit(1)
PY
  while IFS= read -r topic; do
    [[ -z "$topic" ]] && continue
    compose exec -T kafka /opt/kafka/bin/kafka-topics.sh \
      --bootstrap-server 127.0.0.1:9092 \
      --create --if-not-exists --topic "$topic" \
      --partitions 3 --replication-factor 1 >/dev/null
  done <<<"$topics"
}

write_runtime_profile() {
  local gateway_port
  local user_port
  local problem_port
  local problem_grpc_port
  local submit_port
  local judge_port
  local dsn
  dsn="user:password@tcp(127.0.0.1:3306)/fuzoj?parseTime=true&loc=Local"
  read -r gateway_port user_port problem_port problem_grpc_port submit_port judge_port < <(pick_free_ports 6)
  mkdir -p "$OUTPUT_DIR"
  cat >"$OUTPUT_DIR/test.yaml" <<EOF
mysql:
  dsn: "$dsn"
  maxOpenConnections: 25
  maxIdleConnections: 5
  connMaxLifetime: "5m"
  connMaxIdleTime: "10m"
redis:
  addr: "127.0.0.1:6379"
  password: ""
  db: 0
  maxRetries: 3
  minRetryBackoff: "8ms"
  maxRetryBackoff: "512ms"
  dialTimeout: "5s"
  readTimeout: "3s"
  writeTimeout: "3s"
  poolSize: 20
  minIdleConns: 2
  poolTimeout: "4s"
  connMaxIdleTime: "10m"
  connMaxLifetime: "30m"
EOF
  cat >"$PROFILE_PATH" <<EOF
outputDir: "."
services:
  gateway:
    base: "../gateway.yaml"
    output: "gateway.yaml"
    overrides:
      server:
        addr: "0.0.0.0:${gateway_port}"
      redis:
        addr: "127.0.0.1:6379"
        password: ""
        db: 0
      kafka:
        brokers: ["127.0.0.1:9092"]
      upstreams:
        - name: "user"
          baseURL: "http://127.0.0.1:${user_port}"
        - name: "problem"
          baseURL: "http://127.0.0.1:${problem_port}"
        - name: "submit"
          baseURL: "http://127.0.0.1:${submit_port}"
        - name: "judge"
          baseURL: "http://127.0.0.1:${judge_port}"
        - name: "minio"
          baseURL: "http://127.0.0.1:9000"
  user-service:
    base: "../user_service.yaml"
    output: "user_service.yaml"
    overrides:
      server:
        addr: "0.0.0.0:${user_port}"
      database:
        dsn: "$dsn"
      redis:
        addr: "127.0.0.1:6379"
        password: ""
        db: 0
  problem-service:
    base: "../problem_service.yaml"
    output: "problem_service.yaml"
    overrides:
      server:
        addr: "0.0.0.0:${problem_port}"
      grpc:
        addr: "0.0.0.0:${problem_grpc_port}"
      database:
        dsn: "$dsn"
      redis:
        addr: "127.0.0.1:6379"
        password: ""
        db: 0
      kafka:
        brokers: ["127.0.0.1:9092"]
      minio:
        endpoint: "127.0.0.1:9000"
        accessKey: "minioadmin"
        secretKey: "minioadmin"
        useSSL: false
  submit-service:
    base: "../submit_service.yaml"
    output: "submit_service.yaml"
    overrides:
      server:
        addr: "0.0.0.0:${submit_port}"
      database:
        dsn: "$dsn"
      redis:
        addr: "127.0.0.1:6379"
        password: ""
        db: 0
      kafka:
        brokers: ["127.0.0.1:9092"]
      minio:
        endpoint: "127.0.0.1:9000"
        accessKey: "minioadmin"
        secretKey: "minioadmin"
        useSSL: false
  judge-service:
    base: "../judge_service.yaml"
    output: "judge_service.yaml"
    overrides:
      server:
        addr: "0.0.0.0:${judge_port}"
      database:
        dsn: "$dsn"
      redis:
        addr: "127.0.0.1:6379"
        password: ""
        db: 0
      problemRPC:
        addr: "127.0.0.1:${problem_grpc_port}"
      kafka:
        brokers: ["127.0.0.1:9092"]
      minio:
        endpoint: "127.0.0.1:9000"
        accessKey: "minioadmin"
        secretKey: "minioadmin"
        useSSL: false
EOF
}

require_command docker
require_command python3

step "starting dependencies..." compose up -d

step "waiting for mysql..." wait_for_mysql
step "waiting for redis..." wait_for_redis
step "waiting for kafka..." wait_for_kafka
step "waiting for elasticsearch..." wait_for_elasticsearch
step "waiting for kibana..." wait_for_kibana

step "initializing mysql..." init_mysql
step "importing schemas..." import_schema "$ROOT_DIR/internal/user/schema.sql"
step "importing schemas..." import_schema "$ROOT_DIR/internal/problem/schema.sql"
step "importing schemas..." import_schema "$ROOT_DIR/internal/submit/schema.sql"

step "ensuring minio buckets..." wait_for_minio
step "ensuring minio buckets..." ensure_minio_buckets
step "ensuring kafka topics..." ensure_kafka_topics

step "writing runtime profile..." write_runtime_profile

log "dependencies ready"
