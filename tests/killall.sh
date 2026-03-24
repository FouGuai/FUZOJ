#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MANIFEST="${ROOT_DIR}/scripts/debug_manifest.yaml"

python3 "${ROOT_DIR}/scripts/debug_stop.py" --manifest "${MANIFEST}" || true

patterns=(
  "/logs/bin/gateway"
  "/logs/bin/user-service"
  "/logs/bin/problem-service"
  "/logs/bin/submit-service"
  "/logs/bin/status-service"
  "/logs/bin/status-sse-service"
  "/logs/bin/judge-service"
  "/logs/bin/contest-service"
  "/logs/bin/contest-rpc-service"
  "/logs/bin/rank-service"
  "/logs/bin/rank-ws-service"
  "/logs/bin/rank-rpc-service"
)

for pattern in "${patterns[@]}"; do
  if pgrep -f "${pattern}" >/dev/null 2>&1; then
    pkill -f "${pattern}" || true
  fi
done

sleep 1

for pattern in "${patterns[@]}"; do
  if pgrep -f "${pattern}" >/dev/null 2>&1; then
    pkill -9 -f "${pattern}" || true
  fi
done
