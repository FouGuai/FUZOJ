#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_DIR="$ROOT_DIR/logs"
mkdir -p "$LOG_DIR"

run_service() {
  local name="$1"
  local pkg="$2"
  echo "starting $name..."
  (cd "$ROOT_DIR" && nohup go run "$pkg" >"$LOG_DIR/$name.log" 2>&1 & echo $! >"$LOG_DIR/$name.pid")
}

run_service "user-service" "./cmd/user-service"
run_service "problem-service" "./cmd/problem-service"
run_service "submit-service" "./cmd/submit-service"
run_service "judge-service" "./cmd/judge-service"
run_service "gateway" "./cmd/gateway"

echo "all services started"

echo "logs: $LOG_DIR"
