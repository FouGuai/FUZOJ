#!/usr/bin/env python3
import os
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
            print(f"{svc}: running (pid {pid_int})")


if __name__ == "__main__":
    main()
