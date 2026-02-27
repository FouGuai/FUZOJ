#!/usr/bin/env python3
import json
import os
import re
import subprocess
from pathlib import Path

try:
    import yaml
except ImportError as exc:
    raise SystemExit("PyYAML is required. Please install it with: pip install -r requirements.txt") from exc


def main() -> None:
    root_dir = Path(os.environ.get("ROOT_DIR", ".")).resolve()
    manifest_path = root_dir / "scripts/debug_manifest.yaml"
    if not manifest_path.exists():
        raise SystemExit(f"manifest not found: {manifest_path}")

    data = yaml.safe_load(manifest_path.read_text(encoding="utf-8")) or {}
    services = [svc.get("name") for svc in data.get("services", []) if isinstance(svc, dict) and svc.get("name")]
    if not services:
        raise SystemExit("no services found in manifest")

    log_dir = root_dir / data.get("logDir", "logs")
    for svc in services:
        pid_file = log_dir / f"{svc}.pid"
        if not pid_file.exists():
            print(f"{svc}: not running (pid file missing)")
            continue
        pid = pid_file.read_text(encoding="utf-8").strip()
        if not pid:
            print(f"{svc}: not running (pid file empty)")
            continue
        try:
            pid_int = int(pid)
        except ValueError:
            print(f"{svc}: not running (invalid pid {pid})")
            continue
        try:
            os.kill(pid_int, 0)
        except OSError:
            print(f"{svc}: not running (stale pid {pid_int})")
        else:
            ports = extract_ports_from_etcd(svc) or extract_ports(log_dir / f"{svc}.log")
            port_info = format_ports(ports)
            if port_info:
                print(f"{svc}: running (pid {pid_int}, {port_info})")
            else:
                print(f"{svc}: running (pid {pid_int})")


def extract_ports(log_path: Path) -> dict:
    if not log_path.exists():
        return {}
    try:
        lines = log_path.read_text(encoding="utf-8", errors="ignore").splitlines()
    except OSError:
        return {}
    tail = lines[-200:] if len(lines) > 200 else lines
    rest_patterns = [
        re.compile(r"Starting server at ([^\s:]+):(\d+)"),
        re.compile(r"gateway http server started.*\"addr\":\"([^\":]+):(\d+)\""),
        re.compile(r"\"addr\":\"([^\":]+):(\d+)\""),
    ]
    rpc_patterns = [
        re.compile(r"Starting rpc server at ([^\s:]+):(\d+)"),
        re.compile(r"Starting RPC server at ([^\s:]+):(\d+)"),
    ]
    rest = None
    rpc = None
    for line in reversed(tail):
        if rpc is None:
            for pat in rpc_patterns:
                match = pat.search(line)
                if match:
                    rpc = f"{match.group(1)}:{match.group(2)}"
                    break
        if rest is None:
            for pat in rest_patterns:
                match = pat.search(line)
                if match:
                    rest = f"{match.group(1)}:{match.group(2)}"
                    break
        if rest and rpc:
            break
    out = {}
    if rest:
        out["rest"] = rest
    if rpc:
        out["rpc"] = rpc
    return out


def format_ports(ports: dict) -> str:
    if not ports:
        return ""
    parts = []
    rest = ports.get("rest")
    rpc = ports.get("rpc")
    if rest:
        parts.append(f"rest {rest}")
    if rpc:
        parts.append(f"rpc {rpc}")
    return ", ".join(parts)


def extract_ports_from_etcd(service_name: str) -> dict:
    runtime_service = to_runtime_service(service_name)
    if not runtime_service:
        return {}
    rest_key = f"{runtime_service}.rest.runtime"
    rest = read_etcd_runtime(rest_key)
    rpc = None
    if runtime_service in ("problem", "contest.rpc"):
        rpc_key = f"{runtime_service}.rpc.runtime"
        rpc = read_etcd_runtime(rpc_key)
    return build_ports(rest, rpc)


def to_runtime_service(service_name: str) -> str:
    if service_name.endswith("-rpc-service"):
        base = service_name[: -len("-rpc-service")]
        return f"{base}.rpc"
    if service_name.endswith("-service"):
        return service_name[: -len("-service")]
    return service_name


def read_etcd_runtime(key: str) -> dict:
    output = read_etcd_value(key)
    if not output:
        return {}
    try:
        data = json.loads(output)
    except json.JSONDecodeError:
        return {}
    if not isinstance(data, dict):
        return {}
    return data


def read_etcd_value(key: str) -> str:
    cmd = [
        "docker",
        "exec",
        "-t",
        "fuzoj-etcd-1",
        "etcdctl",
        "get",
        key,
        "--print-value-only",
    ]
    try:
        result = subprocess.run(cmd, capture_output=True, text=True, check=False)
    except OSError:
        return ""
    if result.returncode != 0:
        return ""
    return result.stdout.strip()


def build_ports(rest: dict, rpc: dict) -> dict:
    out = {}
    rest = rest or {}
    rpc = rpc or {}
    host = rest.get("host") or rest.get("Host")
    port = rest.get("port") or rest.get("Port")
    if host and port:
        out["rest"] = f"{host}:{port}"
    listen_on = rpc.get("listenOn") or rpc.get("ListenOn")
    if listen_on:
        out["rpc"] = listen_on
    return out


if __name__ == "__main__":
    main()
