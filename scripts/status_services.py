#!/usr/bin/env python3
import argparse
import json
import os
import re
import subprocess
from pathlib import Path
from typing import List, Tuple

try:
    import yaml
except ImportError as exc:
    raise SystemExit("PyYAML is required. Please install it with: pip install -r requirements.txt") from exc


def main() -> None:
    args = parse_args()
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
        instances = collect_instance_pids(log_dir, svc)
        if not instances:
            print(f"{svc}: not running (pid file missing)")
            continue
        running: List[str] = []
        stale: List[str] = []
        invalid: List[str] = []
        for name, pid_value in instances:
            if not pid_value:
                invalid.append(f"{name}=empty")
                continue
            try:
                pid_int = int(pid_value)
            except ValueError:
                invalid.append(f"{name}={pid_value}")
                continue
            try:
                os.kill(pid_int, 0)
            except OSError:
                stale.append(f"{name}={pid_int}")
            else:
                running.append(f"{name}={pid_int}")

        if not running:
            detail = ", ".join(stale + invalid) if stale or invalid else "no live pid"
            print(f"{svc}: down ({detail})")
            continue

        ports = extract_ports_from_etcd(svc) or extract_ports(log_dir / f"{svc}.log")
        targets = extract_discovery_targets(svc)
        if args.verbose:
            port_info = format_ports(ports)
            discovered = format_discovery_targets(targets)
            details = [f"instances {len(running)}"]
            details.append("pids " + ",".join(running))
            if stale:
                details.append("stale " + ",".join(stale))
            if invalid:
                details.append("invalid " + ",".join(invalid))
            if port_info:
                details.append(port_info)
            if discovered:
                details.append(discovered)
            print(f"{svc}: running ({'; '.join(details)})")
            continue

        print(format_concise_line(svc, len(running), ports, targets, stale, invalid))


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Show local debug services status")
    parser.add_argument(
        "--verbose",
        action="store_true",
        help="Show detailed status output (pids, stale/invalid pid files, discovery targets)",
    )
    return parser.parse_args()


def format_concise_line(
    service: str,
    running_count: int,
    ports: dict,
    targets: dict,
    stale: list[str],
    invalid: list[str],
) -> str:
    parts = [f"{service}: up x{running_count}"]
    port_info = format_ports_concise(ports, targets)
    if port_info:
        parts.append(port_info)
    if stale:
        parts.append(f"stale={len(stale)}")
    if invalid:
        parts.append(f"invalid={len(invalid)}")
    return " | ".join(parts)


def collect_instance_pids(log_dir: Path, service_name: str) -> List[Tuple[str, str]]:
    pattern = re.compile(rf"^{re.escape(service_name)}(?:-\d+)?\.pid$")
    paths = sorted(path for path in log_dir.glob("*.pid") if pattern.match(path.name))
    results: List[Tuple[str, str]] = []
    for path in paths:
        instance_name = path.stem
        try:
            pid_value = path.read_text(encoding="utf-8").strip()
        except OSError:
            pid_value = ""
        results.append((instance_name, pid_value))
    return results


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


def format_ports_concise(ports: dict, targets: dict) -> str:
    if not ports and not targets:
        return ""
    parts = []
    rest_targets = (targets or {}).get("rest") or []
    rpc_targets = (targets or {}).get("rpc") or []
    rest = (ports or {}).get("rest")
    rpc = (ports or {}).get("rpc")

    if len(rest_targets) > 1:
        parts.append(f"rest x{len(rest_targets)}")
    elif rest:
        parts.append(f"rest {rest}")
    elif len(rest_targets) == 1:
        parts.append(f"rest {rest_targets[0]}")

    if len(rpc_targets) > 1:
        parts.append(f"rpc x{len(rpc_targets)}")
    elif rpc:
        parts.append(f"rpc {rpc}")
    elif len(rpc_targets) == 1:
        parts.append(f"rpc {rpc_targets[0]}")

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


def extract_discovery_targets(service_name: str) -> dict:
    runtime_service = to_runtime_service(service_name)
    if not runtime_service:
        return {}
    result = {}
    rest_key = f"{runtime_service}.rest"
    rest_targets = read_etcd_values_prefix(rest_key)
    if rest_targets:
        result["rest"] = sorted(set(rest_targets))
    rpc_key = f"{runtime_service}.rpc" if not runtime_service.endswith(".rpc") else runtime_service
    rpc_targets = read_etcd_values_prefix(rpc_key)
    if rpc_targets:
        result["rpc"] = sorted(set(rpc_targets))
    return result


def read_etcd_values_prefix(key: str) -> list[str]:
    cmd = [
        "docker",
        "exec",
        "-t",
        "fuzoj-etcd-1",
        "etcdctl",
        "get",
        key,
        "--prefix",
        "--print-value-only",
    ]
    try:
        result = subprocess.run(cmd, capture_output=True, text=True, check=False)
    except OSError:
        return []
    if result.returncode != 0:
        return []
    addr_pattern = re.compile(r"^[^{}\s]+:\d+$")
    values = []
    for line in result.stdout.splitlines():
        text = line.strip()
        if not text:
            continue
        if not addr_pattern.match(text):
            continue
        values.append(text)
    return values


def format_discovery_targets(targets: dict) -> str:
    if not targets:
        return ""
    parts = []
    rest = targets.get("rest") or []
    rpc = targets.get("rpc") or []
    if rest:
        parts.append("rest targets=" + ",".join(rest))
    if rpc:
        parts.append("rpc targets=" + ",".join(rpc))
    return "; ".join(parts)


if __name__ == "__main__":
    main()
