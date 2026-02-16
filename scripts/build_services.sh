#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_DIR="$ROOT_DIR/logs"
BIN_DIR="$LOG_DIR/bin"
ONLY_SERVICES=""

usage() {
  cat <<'EOF'
Usage: scripts/build_services.sh [options]

Options:
  --bin-dir <path>     Output directory for binaries (default: logs/bin)
  --only <list>        Comma-separated service list to build
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --bin-dir)
      BIN_DIR="$2"
      shift 2
      ;;
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

mkdir -p "$BIN_DIR"

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

build_service() {
  local name="$1"
  local pkg="$2"
  if ! should_run "$name"; then
    return 0
  fi
  echo "building $name..."
  (cd "$ROOT_DIR" && go build -o "$BIN_DIR/$name" "$pkg")
}

build_service "user-service" "./cmd/user-service"
build_service "problem-service" "./cmd/problem-service"
build_service "submit-service" "./cmd/submit-service"
build_service "judge-service" "./cmd/judge-service"
build_service "gateway" "./cmd/gateway"

echo "binaries ready: $BIN_DIR"
