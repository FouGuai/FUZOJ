#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_DIR="$ROOT_DIR/logs"
BIN_DIR="$LOG_DIR/bin"
ONLY_SERVICES=""
PROTO_ROOT="$ROOT_DIR/api/proto"

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

check_tool() {
  local name="$1"
  if ! command -v "$name" >/dev/null 2>&1; then
    echo "missing required tool: $name" >&2
    exit 1
  fi
}

ensure_go_tools_in_path() {
  local goBin
  goBin="$(go env GOPATH 2>/dev/null)/bin"
  if [[ -n "$goBin" && -d "$goBin" ]]; then
    export PATH="$goBin:$PATH"
  fi
}

generate_protos() {
  local -a proto_files
  local modulePath

  if [[ ! -d "$PROTO_ROOT" ]]; then
    return 0
  fi

  mapfile -t proto_files < <(find "$PROTO_ROOT" -type f -name '*.proto')
  if [[ ${#proto_files[@]} -eq 0 ]]; then
    return 0
  fi

  ensure_go_tools_in_path
  check_tool protoc
  check_tool protoc-gen-go
  check_tool protoc-gen-go-grpc

  modulePath="$(awk '$1 == "module" {print $2; exit}' "$ROOT_DIR/go.mod")"
  if [[ -z "$modulePath" ]]; then
    echo "failed to resolve module path from go.mod" >&2
    exit 1
  fi

  echo "generating protobuf code..."
  (cd "$ROOT_DIR" && \
    protoc -I "$PROTO_ROOT" \
      --go_out=. --go_opt="module=$modulePath" \
      --go-grpc_out=. --go-grpc_opt="module=$modulePath" \
      "${proto_files[@]}")
}

generate_protos
build_service "user-service" "./cmd/user-service"
build_service "problem-service" "./cmd/problem-service"
build_service "submit-service" "./cmd/submit-service"
build_service "judge-service" "./cmd/judge-service"
build_service "gateway" "./cmd/gateway"

echo "binaries ready: $BIN_DIR"
