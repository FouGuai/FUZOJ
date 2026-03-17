#!/usr/bin/env python3
import argparse
import os
import re
import signal
import subprocess
import time
from pathlib import Path
from typing import Dict, List

try:
    import yaml
except ImportError as exc:
    raise SystemExit("PyYAML is required. Please install it with: pip install -r requirements.txt") from exc


def find_repo_root(start: Path) -> Path:
    current = start.resolve()
    for parent in [current] + list(current.parents):
        if (parent / "go.mod").exists():
            return parent
    return current


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
    pattern = re.compile(rf"^{re.escape(name)}(?:-\d+)?\.pid$")
    pid_paths = sorted(path for path in log_dir.glob("*.pid") if pattern.match(path.name))
    if not pid_paths:
        return
    for pid_path in reversed(pid_paths):
        stop_service_pid(pid_path, grace_s)


def stop_service_pid(pid_path: Path, grace_s: float) -> None:
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
    if not manifest_path.is_absolute():
        candidates = []
        candidates.append((Path.cwd() / manifest_path).resolve())
        if manifest_path.parts and manifest_path.parts[0] == "scripts":
            candidates.append((Path(__file__).resolve().parent / Path(*manifest_path.parts[1:])).resolve())
        candidates.append((Path(__file__).resolve().parent / manifest_path).resolve())
        for candidate in candidates:
            if candidate.exists():
                manifest_path = candidate
                break
        else:
            manifest_path = candidates[-1]
    manifest = load_manifest(manifest_path)
    root_base = find_repo_root(manifest_path.parent)
    root_dir = Path(manifest.get("rootDir", "."))
    if not root_dir.is_absolute():
        root = (root_base / root_dir).resolve()
    else:
        root = root_dir.resolve()
    log_dir = root / manifest.get("logDir", "logs")

    only_set = {name.strip() for name in args.only.split(",") if name.strip()}

    if not args.deps_only:
        services = manifest.get("services", [])
        if not isinstance(services, list):
            raise SystemExit("manifest services must be a list")
        selected = [svc for svc in services if not only_set or svc["name"] in only_set]
        for svc in reversed(selected):
            stop_service(log_dir, svc["name"], args.grace_seconds)

    if args.services_only:
        return

    base_cmd = compose_base(manifest, root)
    env = compose_env(manifest)
    subprocess.run(base_cmd + ["down"], env=env, check=False)


if __name__ == "__main__":
    main()
