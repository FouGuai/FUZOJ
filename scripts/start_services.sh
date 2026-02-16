#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_DIR="$ROOT_DIR/logs"
BIN_DIR="$LOG_DIR/bin"
PROFILE_PATH="configs/dev-profile.yaml"
OUTPUT_DIR="configs/dev.generated"
ONLY_SERVICES=""
NO_GEN="false"
NO_BUILD="false"

mkdir -p "$LOG_DIR"

usage() {
  cat <<'EOF'
Usage: scripts/start_services.sh [options]

Options:
  --profile <path>     Config profile path (default: configs/dev-profile.yaml)
  --output-dir <path>  Override output directory (default: configs/dev.generated)
  --only <list>        Comma-separated service list to start
  --no-gen             Skip config generation
  --no-build           Skip binary build
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --profile)
      PROFILE_PATH="$2"
      shift 2
      ;;
    --output-dir)
      OUTPUT_DIR="$2"
      shift 2
      ;;
    --only)
      ONLY_SERVICES="$2"
      shift 2
      ;;
    --no-gen)
      NO_GEN="true"
      shift 1
      ;;
    --no-build)
      NO_BUILD="true"
      shift 1
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

generate_configs() {
  if [[ "$NO_GEN" == "true" ]]; then
    return 0
  fi
  (cd "$ROOT_DIR" && go run ./scripts/devtools/configgen -profile "$PROFILE_PATH" -output-dir "$OUTPUT_DIR")
}

build_services() {
  if [[ "$NO_BUILD" == "true" ]]; then
    return 0
  fi
  "$ROOT_DIR/scripts/build_services.sh" --bin-dir "$BIN_DIR" --only "$ONLY_SERVICES"
}

run_service() {
  local name="$1"
  local config="$2"
  local pid_file="$LOG_DIR/$name.pid"
  local bin_path="$BIN_DIR/$name"
  local config_path="$config"
  if [[ "$config_path" != /* ]]; then
    config_path="$ROOT_DIR/$config_path"
  fi
  if ! should_run "$name"; then
    return 0
  fi
  if [[ ! -f "$config_path" ]]; then
    echo "config file not found: $config_path" >&2
    exit 1
  fi
  if [[ ! -x "$bin_path" ]]; then
    echo "binary not found or not executable: $bin_path" >&2
    exit 1
  fi
  echo "starting $name..."
  (cd "$ROOT_DIR" && nohup "$bin_path" -config "$config_path" >"$LOG_DIR/$name.log" 2>&1 & echo $! >"$pid_file")
  if [[ -f "$pid_file" ]]; then
    local pid
    pid="$(cat "$pid_file")"
    echo "$name pid: $pid"
  fi
}

generate_configs
build_services

run_service "user-service" "$OUTPUT_DIR/user_service.yaml"
run_service "problem-service" "$OUTPUT_DIR/problem_service.yaml"
run_service "submit-service" "$OUTPUT_DIR/submit_service.yaml"
run_service "judge-service" "$OUTPUT_DIR/judge_service.yaml"
run_service "gateway" "$OUTPUT_DIR/gateway.yaml"

print_http_ports() {
  local name="$1"
  local config="$2"
  local config_path="$config"
  if [[ "$config_path" != /* ]]; then
    config_path="$ROOT_DIR/$config_path"
  fi
  if ! should_run "$name"; then
    return 0
  fi
  if [[ ! -f "$config_path" ]]; then
    return 0
  fi
  local addr
  addr="$(awk '
    /^server:/ {in_server=1; next}
    in_server && $1 == "addr:" {print $2; exit}
    in_server && /^[^[:space:]]/ {in_server=0}
  ' "$config_path")"
  if [[ -z "$addr" ]]; then
    return 0
  fi
  addr="${addr%\"}"
  addr="${addr#\"}"
  local port="${addr##*:}"
  echo "$name http addr: $addr (port $port)"
}

echo "services started"
echo "logs: $LOG_DIR"
print_http_ports "user-service" "$OUTPUT_DIR/user_service.yaml"
print_http_ports "problem-service" "$OUTPUT_DIR/problem_service.yaml"
print_http_ports "submit-service" "$OUTPUT_DIR/submit_service.yaml"
print_http_ports "judge-service" "$OUTPUT_DIR/judge_service.yaml"
print_http_ports "gateway" "$OUTPUT_DIR/gateway.yaml"
