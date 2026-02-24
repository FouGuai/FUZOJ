#!/usr/bin/env python3
import argparse
import os
import signal
import subprocess
import time
from pathlib import Path
from typing import Dict, List

try:
    import yaml
except ImportError as exc:
    raise SystemExit("PyYAML is required. Please install it with: pip install -r requirements.txt") from exc


def load_manifest(path: Path) -> Dict:
    if not path.exists():
        raise SystemExit(f"manifest not found: {path}")
    data = yaml.safe_load(path.read_text(encoding="utf-8"))
    if not isinstance(data, dict):
        raise SystemExit("manifest root must be a mapping")
    return data


def compose_base(manifest: Dict, root: Path) -> List[str]:
    compose_file = root / manifest["deps"]["composeFile"]
    return ["docker", "compose", "--ansi", "never", "-f", str(compose_file)]


def compose_env(manifest: Dict) -> Dict[str, str]:
    env = os.environ.copy()
    project = manifest.get("deps", {}).get("projectName")
    if project:
        env["COMPOSE_PROJECT_NAME"] = project
    return env


def is_pid_running(pid: int) -> bool:
    try:
        os.kill(pid, 0)
        return True
    except OSError:
        return False


def stop_service(log_dir: Path, name: str, grace_s: float) -> None:
    pid_path = log_dir / f"{name}.pid"
    if not pid_path.exists():
        return
    try:
        pid = int(pid_path.read_text(encoding="utf-8").strip())
    except ValueError:
        pid = -1

    if pid <= 0 or not is_pid_running(pid):
        pid_path.unlink(missing_ok=True)
        return

    try:
        os.kill(pid, signal.SIGTERM)
    except OSError:
        pid_path.unlink(missing_ok=True)
        return

    deadline = time.time() + grace_s
    while time.time() < deadline:
        if not is_pid_running(pid):
            pid_path.unlink(missing_ok=True)
            return
        time.sleep(0.2)

    try:
        os.kill(pid, signal.SIGKILL)
    except OSError:
        pass
    pid_path.unlink(missing_ok=True)


def main() -> None:
    parser = argparse.ArgumentParser(description="Stop debug deps and services")
    parser.add_argument("--manifest", default="scripts/debug_manifest.yaml", help="Path to manifest")
    parser.add_argument("--only", default="", help="Comma-separated service list")
    parser.add_argument("--deps-only", action="store_true", help="Only stop dependencies")
    parser.add_argument("--services-only", action="store_true", help="Only stop services")
    parser.add_argument("--grace-seconds", type=float, default=4.0, help="Graceful shutdown timeout")
    args = parser.parse_args()

    manifest_path = Path(args.manifest)
    manifest = load_manifest(manifest_path)
    root = (manifest_path.parent / manifest.get("rootDir", ".")).resolve()
    log_dir = root / manifest.get("logDir", "logs")

    only_set = {name.strip() for name in args.only.split(",") if name.strip()}

    if not args.deps_only:
        services = manifest.get("services", [])
        if not isinstance(services, list):
            raise SystemExit("manifest services must be a list")
        for svc in services:
            name = svc["name"]
            if only_set and name not in only_set:
                continue
            stop_service(log_dir, name, args.grace_seconds)

    if args.services_only:
        return

    base_cmd = compose_base(manifest, root)
    env = compose_env(manifest)
    subprocess.run(base_cmd + ["down"], env=env, check=False)


if __name__ == "__main__":
    main()
