#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/deploy/docker-compose.deps.yml"
COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-fuzoj}"

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

compose() {
  COMPOSE_PROJECT_NAME="$COMPOSE_PROJECT_NAME" docker compose --ansi=never -f "$COMPOSE_FILE" "$@"
}

if ! command -v docker >/dev/null 2>&1; then
  echo "missing required tool: docker" >&2
  exit 1
fi

compose stop elasticsearch kibana filebeat >/dev/null 2>&1 || true
compose down -v
