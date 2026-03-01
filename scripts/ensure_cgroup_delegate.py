#!/usr/bin/env python3
import argparse
import os
import shlex
import shutil
import subprocess
from pathlib import Path
from typing import List, Optional


def find_cgroup2_mount() -> Optional[Path]:
    try:
        with open("/proc/self/mountinfo", "r", encoding="utf-8") as handle:
            for line in handle:
                parts = line.strip().split(" - ")
                if len(parts) != 2:
                    continue
                post = parts[1].split()
                if not post or post[0] != "cgroup2":
                    continue
                pre = parts[0].split()
                if len(pre) >= 5:
                    return Path(pre[4])
    except OSError:
        return None
    return None


def current_cgroup_subpath() -> Optional[str]:
    try:
        raw = Path("/proc/self/cgroup").read_text(encoding="utf-8")
    except OSError:
        return None
    for line in raw.splitlines():
        if line.startswith("0::"):
            return line.split("::", 1)[1] or "/"
    return None


def user_service_cgroup_base() -> Optional[Path]:
    mount = find_cgroup2_mount()
    if mount is None:
        return None
    uid = os.getuid()
    base = mount / "user.slice" / f"user-{uid}.slice" / f"user@{uid}.service"
    if base.exists():
        return base
    return None


def app_slice_cgroup_base() -> Optional[Path]:
    base = user_service_cgroup_base()
    if base is None:
        return None
    app_slice = base / "app.slice"
    if app_slice.exists():
        return app_slice
    return None


def should_skip_cgroup_base(path: Path) -> bool:
    name = path.name
    if not name:
        return False
    if name.endswith(".scope"):
        return True
    if name == "init.scope":
        return True
    return False


def collect_probe_bases() -> List[Path]:
    bases: List[Path] = []

    def append_base(path: Optional[Path]) -> None:
        if path is None or not path.exists():
            return
        if should_skip_cgroup_base(path):
            return
        if path not in bases:
            bases.append(path)

    service_base = user_service_cgroup_base()
    append_base(service_base)
    append_base(app_slice_cgroup_base())

    mount = find_cgroup2_mount()
    subpath = current_cgroup_subpath()
    if mount is None or subpath is None:
        return bases
    if subpath in ("", "/"):
        current = mount
    else:
        current = mount / subpath.lstrip("/")
    while current.exists():
        append_base(current)
        if service_base is not None and current == service_base:
            break
        if current == mount:
            break
        parent = current.parent
        if parent == current:
            break
        current = parent

    return bases


def can_write_pids_limit(prefix: str) -> bool:
    for base in collect_probe_bases():
        target = base / f"{prefix}-{os.getpid()}"
        try:
            target.mkdir(parents=True, exist_ok=False)
        except OSError:
            continue
        try:
            pids_file = target / "pids.max"
            pids_file.write_text("max", encoding="utf-8")
            return True
        except OSError:
            continue
        finally:
            try:
                target.rmdir()
            except OSError:
                pass
    return False


def exec_command(command: List[str]) -> None:
    os.execvp(command[0], command)


def run_in_delegated_scope(command: List[str]) -> None:
    systemd_run = shutil.which("systemd-run")
    if not systemd_run:
        raise SystemExit("systemd-run not found; cannot auto-delegate cgroup")
    cwd = str(Path.cwd())
    cmd_line = f"cd {shlex.quote(cwd)} && exec {shlex.join(command)}"
    cmd = [
        systemd_run,
        "--user",
        "--scope",
        "-p",
        "Delegate=yes",
        "-p",
        "TasksAccounting=yes",
        "-p",
        "CPUAccounting=yes",
        "-p",
        "MemoryAccounting=yes",
        "--",
        "bash",
        "-lc",
        cmd_line,
    ]
    result = subprocess.run(cmd, check=False)
    raise SystemExit(result.returncode)


def main() -> None:
    parser = argparse.ArgumentParser(description="Ensure delegated cgroup scope before running a command")
    parser.add_argument("--prefix", default="fuzoj-debug", help="Sub-cgroup prefix to probe")
    parser.add_argument("command", nargs=argparse.REMAINDER, help="Command to run after delegation")
    args = parser.parse_args()

    command = list(args.command)
    if command and command[0] == "--":
        command = command[1:]
    if not command:
        raise SystemExit("command is required; use -- <command> [args]")

    if os.geteuid() == 0 or can_write_pids_limit(args.prefix):
        exec_command(command)

    run_in_delegated_scope(command)


if __name__ == "__main__":
    main()
