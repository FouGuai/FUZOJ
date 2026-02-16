#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_DIR="$ROOT_DIR/logs"
ONLY_SERVICES=""

usage() {
  cat <<'EOF'
Usage: scripts/stop_services.sh [options]

Options:
  --only <list>        Comma-separated service list to stop
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --only)
      ONLY_SERVICES="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

should_run() {
  local name="$1"
  if [[ -z "$ONLY_SERVICES" ]]; then
    return 0
  fi
  IFS=',' read -r -a items <<<"$ONLY_SERVICES"
  for item in "${items[@]}"; do
    if [[ "$item" == "$name" ]]; then
      return 0
    fi
  done
  return 1
}

stop_service() {
  local name="$1"
  if ! should_run "$name"; then
    return 0
  fi
  local pid_file="$LOG_DIR/$name.pid"
  if [[ ! -f "$pid_file" ]]; then
    echo "skip $name: pid file not found"
    return 0
  fi
  local pid
  pid="$(cat "$pid_file" | tr -d '[:space:]')"
  if [[ -z "$pid" ]]; then
    echo "skip $name: pid file empty"
    rm -f "$pid_file"
    return 0
  fi
  if ! kill -0 "$pid" 2>/dev/null; then
    echo "skip $name: process not running (pid $pid)"
    rm -f "$pid_file"
    return 0
  fi
  echo "stopping $name (pid $pid)..."
  kill "$pid" 2>/dev/null || true
  for _ in {1..20}; do
    if ! kill -0 "$pid" 2>/dev/null; then
      rm -f "$pid_file"
      echo "$name stopped"
      return 0
    fi
    sleep 0.2
  done
  echo "force stopping $name (pid $pid)..."
  kill -9 "$pid" 2>/dev/null || true
  rm -f "$pid_file"
}

stop_service "gateway"
stop_service "judge-service"
stop_service "submit-service"
stop_service "problem-service"
stop_service "user-service"

echo "services stopped"
