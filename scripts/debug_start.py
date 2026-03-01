#!/usr/bin/env python3
import argparse
import json
import os
import shutil
import subprocess
import tempfile
import time
from pathlib import Path
from typing import Dict, List, Optional

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


def collect_rootless_cgroup_bases() -> List[Path]:
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


def enable_cgroup_controllers(cgroup_path: Path, controllers: List[str]) -> None:
    ctrl_file = cgroup_path / "cgroup.subtree_control"
    if not ctrl_file.exists():
        return
    try:
        current = ctrl_file.read_text(encoding="utf-8").strip().split()
        missing = [name for name in controllers if name not in current]
        if not missing:
            return
        ctrl_file.write_text(" ".join(f"+{name}" for name in missing), encoding="utf-8")
    except OSError as exc:
        print(f"warning: unable to enable cgroup controllers in {cgroup_path}: {exc}")


def can_write_pids_max(cgroup_path: Path) -> bool:
    pids_file = cgroup_path / "pids.max"
    if not pids_file.exists():
        return False
    try:
        pids_file.write_text("max", encoding="utf-8")
    except OSError as exc:
        print(f"warning: unable to write pids.max in {cgroup_path}: {exc}")
        return False
    return True


def resolve_rootless_cgroup(prefix: str) -> Optional[Path]:
    for base_path in collect_rootless_cgroup_bases():
        enable_cgroup_controllers(base_path, ["cpu", "memory", "pids"])
        target = base_path / prefix
        target_exists_before = target.exists()
        try:
            target.mkdir(parents=True, exist_ok=True)
        except OSError as exc:
            print(f"warning: unable to create cgroup directory {target}: {exc}")
            continue
        enable_cgroup_controllers(target, ["cpu", "memory", "pids"])
        if not can_write_pids_max(target):
            if not target_exists_before:
                try:
                    target.rmdir()
                except OSError:
                    pass
            continue
        return target
    return None


def prepare_cgroup_root(path: Path) -> Path:
    try:
        path.mkdir(parents=True, exist_ok=True)
    except OSError as exc:
        raise SystemExit(f"failed to create cgroup root {path}: {exc}") from exc
    parent = path.parent
    if parent.exists():
        enable_cgroup_controllers(parent, ["cpu", "memory", "pids"])
    if not (path / "cgroup.controllers").exists():
        print(f"warning: {path} does not look like a cgroup v2 directory")
    enable_cgroup_controllers(path, ["cpu", "memory", "pids"])
    return path


def patch_judge_cgroup_root(config_path: Path, cgroup_root: Path) -> None:
    data = yaml.safe_load(config_path.read_text(encoding="utf-8")) or {}
    if not isinstance(data, dict):
        raise SystemExit("judge config root must be a mapping")
    sandbox = data.get("Sandbox")
    if sandbox is None:
        sandbox = {}
    if not isinstance(sandbox, dict):
        raise SystemExit("judge sandbox config must be a mapping")
    sandbox["CgroupRoot"] = str(cgroup_root)
    data["Sandbox"] = sandbox
    config_path.write_text(yaml.safe_dump(data, sort_keys=False), encoding="utf-8")


def prepare_etcd_config_dir(root: Path, base_dir: Path, cgroup_root: Optional[Path]) -> Path:
    if cgroup_root is None:
        return base_dir
    if not base_dir.exists():
        raise SystemExit(f"etcd config dir not found: {base_dir}")
    temp_root = root / "tmp"
    temp_root.mkdir(parents=True, exist_ok=True)
    temp_dir = Path(tempfile.mkdtemp(prefix="etcdinit-", dir=str(temp_root)))
    shutil.copytree(base_dir, temp_dir, dirs_exist_ok=True)
    judge_cfg = temp_dir / "judge_service/etc/judge.yaml"
    if judge_cfg.exists():
        patch_judge_cgroup_root(judge_cfg, cgroup_root)
    else:
        print("warning: judge config not found in etcd init dir; cgroup override skipped")
    return temp_dir


def run(
    cmd: List[str],
    *,
    env: Optional[Dict[str, str]] = None,
    check: bool = True,
    capture: bool = False,
    input_data: Optional[bytes] = None,
    cwd: Optional[Path] = None,
) -> subprocess.CompletedProcess:
    stdout = subprocess.PIPE if capture else None
    stderr = subprocess.STDOUT if capture else None
    return subprocess.run(
        cmd,
        env=env,
        check=check,
        stdout=stdout,
        stderr=stderr,
        input=input_data,
        cwd=str(cwd) if cwd else None,
    )


def compose_up(base_cmd: List[str], env: Dict[str, str], pull_policy: str) -> None:
    cmd = base_cmd + ["up", "-d"]
    if pull_policy:
        cmd += ["--pull", pull_policy]
    result = run(cmd, env=env, check=False)
    if result.returncode == 0:
        return
    if pull_policy != "never":
        retry_cmd = base_cmd + ["up", "-d", "--pull", "never"]
        retry_result = run(retry_cmd, env=env, check=False)
        if retry_result.returncode == 0:
            return
    raise subprocess.CalledProcessError(result.returncode, cmd)


def compose_base(manifest: Dict, root: Path) -> List[str]:
    compose_file = root / manifest["deps"]["composeFile"]
    return ["docker", "compose", "--ansi", "never", "-f", str(compose_file)]


def compose_env(manifest: Dict) -> Dict[str, str]:
    env = os.environ.copy()
    project = manifest.get("deps", {}).get("projectName")
    if project:
        env["COMPOSE_PROJECT_NAME"] = project
    return env


def wait_for_http(url: str, timeout_s: int = 60, interval_s: float = 1.0) -> None:
    import urllib.request

    deadline = time.time() + timeout_s
    while time.time() < deadline:
        try:
            with urllib.request.urlopen(url, timeout=2) as resp:
                if 200 <= resp.status < 500:
                    return
        except Exception:
            pass
        time.sleep(interval_s)
    raise SystemExit(f"http check failed for {url}")


def wait_for_mysql(base_cmd: List[str], env: Dict[str, str]) -> None:
    cmd = base_cmd + ["exec", "-T", "mysql", "mysqladmin", "ping", "-h", "127.0.0.1", "-uroot", "-proot"]
    for _ in range(180):
        if run(cmd, env=env, check=False).returncode == 0:
            return
        time.sleep(1)
    raise SystemExit("mysql did not become ready in time")


def wait_for_redis(base_cmd: List[str], env: Dict[str, str]) -> None:
    cmd = base_cmd + ["exec", "-T", "redis", "redis-cli", "ping"]
    for _ in range(60):
        if run(cmd, env=env, check=False).returncode == 0:
            return
        time.sleep(1)
    raise SystemExit("redis did not become ready in time")


def wait_for_kafka(base_cmd: List[str], env: Dict[str, str]) -> None:
    cmd = base_cmd + ["exec", "-T", "kafka", "/opt/kafka/bin/kafka-topics.sh", "--bootstrap-server", "127.0.0.1:9092", "--list"]
    for _ in range(60):
        if run(cmd, env=env, check=False).returncode == 0:
            return
        time.sleep(1)
    raise SystemExit("kafka did not become ready in time")


def init_mysql(base_cmd: List[str], env: Dict[str, str]) -> None:
    sql = (
        "CREATE DATABASE IF NOT EXISTS fuzoj CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;"
        "\nCREATE USER IF NOT EXISTS 'user'@'%' IDENTIFIED BY 'password';"
        "\nGRANT ALL PRIVILEGES ON fuzoj.* TO 'user'@'%';"
        "\nFLUSH PRIVILEGES;\n"
    )
    cmd = base_cmd + ["exec", "-T", "mysql", "mysql", "-h", "127.0.0.1", "-uroot", "-proot"]
    run(cmd, env=env, input_data=sql.encode("utf-8"))


def import_schema(base_cmd: List[str], env: Dict[str, str], schema_path: Path) -> None:
    if not schema_path.exists():
        raise SystemExit(f"schema file not found: {schema_path}")
    cmd = base_cmd + ["exec", "-T", "mysql", "mysql", "-uroot", "-proot", "fuzoj"]
    run(cmd, env=env, input_data=schema_path.read_bytes())


def ensure_kafka_topics(base_cmd: List[str], env: Dict[str, str], root: Path, script_path: str) -> None:
    script = root / script_path
    if not script.exists():
        raise SystemExit(f"kafka topics script not found: {script}")
    result = run(["python3", str(script)], capture=True, check=False)
    if result.returncode != 0:
        output = (result.stdout or b"").decode("utf-8", errors="ignore")
        raise SystemExit(f"kafka topics script failed: {output.strip()}")
    topics = (result.stdout or b"").decode("utf-8").strip().split()
    if not topics:
        raise SystemExit("no kafka topics found in configs")
    for topic in topics:
        cmd = base_cmd + [
            "exec",
            "-T",
            "kafka",
            "/opt/kafka/bin/kafka-topics.sh",
            "--bootstrap-server",
            "127.0.0.1:9092",
            "--create",
            "--if-not-exists",
            "--topic",
            topic,
            "--partitions",
            "3",
            "--replication-factor",
            "1",
        ]
        run(cmd, env=env, check=False)


def ensure_minio_buckets(project: str, minio_cfg: Dict) -> None:
    network = f"{project}_default"
    endpoint = minio_cfg["endpoint"]
    access_key = minio_cfg["accessKey"]
    secret_key = minio_cfg["secretKey"]
    for bucket in minio_cfg.get("buckets", []):
        cmd = [
            "docker",
            "run",
            "--rm",
            "--network",
            network,
            "-e",
            f"MC_HOST_local=http://{access_key}:{secret_key}@{endpoint.replace('http://', '').replace('https://', '')}",
            "minio/mc",
            "mb",
            "--ignore-existing",
            f"local/{bucket}",
        ]
        run(cmd, check=False)


def init_etcd(root: Path, config_dir: Path) -> None:
    script = root / "scripts/devtools/etcdinit/main.go"
    if not script.exists():
        raise SystemExit(f"etcd init script not found: {script}")
    cmd = ["go", "run", str(script), "--config-dir", str(config_dir)]
    run(cmd, check=True)


def read_etcd_runtime(base_cmd: List[str], env: Dict[str, str], key: str) -> Optional[Dict]:
    cmd = base_cmd + ["exec", "-T", "etcd", "etcdctl", "get", key, "--print-value-only"]
    result = run(cmd, env=env, capture=True, check=False)
    if result.returncode != 0:
        return None
    raw = (result.stdout or b"").decode("utf-8", errors="ignore").strip()
    if not raw:
        return None
    try:
        return json.loads(raw)
    except json.JSONDecodeError:
        return None


def update_cli_config(root: Path, base_cmd: List[str], env: Dict[str, str], gateway_cfg: Path) -> None:
    host = None
    port = None
    runtime = read_etcd_runtime(base_cmd, env, "gateway.rest.runtime")
    if isinstance(runtime, dict):
        host = runtime.get("host") or runtime.get("Host")
        port = runtime.get("port") or runtime.get("Port")
    if not host or not port:
        candidates = [gateway_cfg, root / "configs/etcdinit/gateway_service/etc/gateway.yaml"]
        for path in candidates:
            if not path.exists():
                continue
            data = yaml.safe_load(path.read_text(encoding="utf-8")) or {}
            host = data.get("Host") or data.get("host")
            port = data.get("Port") or data.get("port")
            if host and port:
                break
    if not host or not port:
        return
    host_value = str(host)
    if host_value in ("0.0.0.0", "::"):
        host_value = "127.0.0.1"
    base_url = f"http://{host_value}:{port}"
    cli_path = root / "configs/cli.yaml"
    if not cli_path.exists():
        return
    cli_cfg = yaml.safe_load(cli_path.read_text(encoding="utf-8")) or {}
    cli_cfg["baseURL"] = base_url
    cli_path.write_text(yaml.safe_dump(cli_cfg, sort_keys=False), encoding="utf-8")


def is_pid_running(pid: int) -> bool:
    try:
        os.kill(pid, 0)
        return True
    except OSError:
        return False


def start_service(
    root: Path,
    log_dir: Path,
    bin_dir: Path,
    service: Dict,
    *,
    allow_rootless: bool,
) -> None:
    name = service["name"]
    pid_path = log_dir / f"{name}.pid"
    if pid_path.exists():
        try:
            pid = int(pid_path.read_text(encoding="utf-8").strip())
        except ValueError:
            pid = -1
        if pid > 0 and is_pid_running(pid):
            raise SystemExit(f"service already running: {name} (pid {pid})")
        pid_path.unlink(missing_ok=True)

    config_path = root / service["config"]
    if not config_path.exists():
        raise SystemExit(f"config file not found: {config_path}")

    bin_path = bin_dir / name
    if not bin_path.exists():
        raise SystemExit(f"binary not found: {bin_path}")

    log_path = log_dir / f"{name}.log"
    cmd = [str(bin_path), "-f", str(config_path)]
    if service.get("runAsRoot") and os.geteuid() != 0 and not allow_rootless:
        raise SystemExit(
            f"{name} requires root or a delegated cgroup v2 subtree; "
            "configure Sandbox.CgroupRoot to a writable cgroup and try again"
        )
    with log_path.open("ab") as log_file:
        proc = subprocess.Popen(
            cmd,
            cwd=str(root),
            stdin=subprocess.DEVNULL,
            stdout=log_file,
            stderr=log_file,
            start_new_session=True,
        )
    pid_path.write_text(str(proc.pid), encoding="utf-8")
    time.sleep(0.2)
    if not is_pid_running(proc.pid):
        raise SystemExit(f"service failed to start: {name}")


def build_service(root: Path, bin_dir: Path, service: Dict) -> None:
    name = service["name"]
    pkg = service["build"]
    cmd = ["go", "build", "-gcflags=all=-N -l", "-o", str(bin_dir / name), pkg]
    run(cmd, env=os.environ.copy(), check=True, capture=False, input_data=None, cwd=root)


def build_sandbox_helper(root: Path, bin_dir: Path) -> None:
    cmd = [
        "go",
        "build",
        "-gcflags=all=-N -l",
        "-o",
        str(bin_dir / "sandbox-init"),
        "./cmd/sandbox-init",
    ]
    run(cmd, env=os.environ.copy(), check=True, capture=False, input_data=None, cwd=root)


def main() -> None:
    parser = argparse.ArgumentParser(description="Start debug deps and services")
    parser.add_argument("--manifest", default="scripts/debug_manifest.yaml", help="Path to manifest")
    parser.add_argument("--no-deps", action="store_true", help="Skip starting dependencies")
    parser.add_argument("--no-build", action="store_true", help="Skip building binaries")
    parser.add_argument("--deps-only", action="store_true", help="Only start dependencies")
    parser.add_argument("--services-only", action="store_true", help="Only start services")
    parser.add_argument("--no-etcd-init", action="store_true", help="Skip etcd config initialization")
    parser.add_argument(
        "--cgroup-root",
        default="",
        help="Override Sandbox.CgroupRoot for rootless judge service",
    )
    parser.add_argument("--only", default="", help="Comma-separated service list")
    parser.add_argument(
        "--pull",
        default="missing",
        choices=["always", "missing", "never"],
        help="Docker compose pull policy",
    )
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
    bin_dir = root / manifest.get("binDir", "logs/bin")
    log_dir.mkdir(parents=True, exist_ok=True)
    bin_dir.mkdir(parents=True, exist_ok=True)

    only_set = {name.strip() for name in args.only.split(",") if name.strip()}
    services = manifest.get("services", [])
    if not isinstance(services, list):
        raise SystemExit("manifest services must be a list")
    active_services = [svc for svc in services if not only_set or svc["name"] in only_set]

    needs_rootless = not args.deps_only and os.geteuid() != 0 and any(svc.get("runAsRoot") for svc in active_services)
    cgroup_root = None
    if args.cgroup_root:
        cgroup_root = prepare_cgroup_root(Path(args.cgroup_root).expanduser())
    elif needs_rootless:
        cgroup_root = resolve_rootless_cgroup("fuzoj-debug")
        if cgroup_root is None:
            raise SystemExit(
                "unable to resolve a writable cgroup v2 root for rootless services; "
                "set --cgroup-root or update Sandbox.CgroupRoot in etcd"
            )
    if cgroup_root is not None:
        print(f"using cgroup root: {cgroup_root}")

    base_cmd = compose_base(manifest, root)
    env = compose_env(manifest)

    should_init_etcd = not args.no_deps and not args.services_only and not args.no_etcd_init
    etcd_config_dir = root / "configs/etcdinit"
    if cgroup_root is not None and should_init_etcd:
        etcd_config_dir = prepare_etcd_config_dir(root, etcd_config_dir, cgroup_root)
        print(f"using etcd config dir: {etcd_config_dir}")
    elif cgroup_root is not None and not should_init_etcd:
        print(
            "warning: rootless cgroup detected but etcd init is disabled; "
            "ensure judge config in etcd sets Sandbox.CgroupRoot to the cgroup root"
        )

    if not args.no_deps and not args.services_only:
        compose_up(base_cmd, env, args.pull)
        wait_for_mysql(base_cmd, env)
        wait_for_redis(base_cmd, env)
        wait_for_kafka(base_cmd, env)
        wait_for_http(manifest["deps"]["etcd"]["healthURL"], timeout_s=60)
        wait_for_http(manifest["deps"]["elasticsearch"]["healthURL"], timeout_s=90)
        wait_for_http(manifest["deps"]["kibana"]["healthURL"], timeout_s=120)
        wait_for_http(manifest["deps"]["minio"]["healthURL"], timeout_s=60)

        if not args.no_etcd_init:
            init_etcd(root, etcd_config_dir)

        init_mysql(base_cmd, env)
        for schema in manifest["deps"]["mysql"].get("schemas", []):
            import_schema(base_cmd, env, root / schema)

        project = manifest["deps"].get("projectName", "fuzoj")
        ensure_minio_buckets(project, manifest["deps"]["minio"])
        ensure_kafka_topics(base_cmd, env, root, manifest["deps"]["kafka"]["ensureTopicsScript"])

    if args.deps_only:
        return

    if not args.no_build:
        build_sandbox_helper(root, bin_dir)
        for svc in services:
            if only_set and svc["name"] not in only_set:
                continue
            build_service(root, bin_dir, svc)

    for svc in services:
        if only_set and svc["name"] not in only_set:
            continue
        start_service(root, log_dir, bin_dir, svc, allow_rootless=cgroup_root is not None)

    gateway_config = None
    for svc in services:
        if svc.get("name") == "gateway":
            gateway_config = root / svc["config"]
            break
    if gateway_config is not None:
        update_cli_config(root, base_cmd, env, gateway_config)

    print("services started")
    print(f"logs: {log_dir}")


def find_repo_root(start: Path) -> Path:
    current = start.resolve()
    for parent in [current] + list(current.parents):
        if (parent / "go.mod").exists():
            return parent
    return current


if __name__ == "__main__":
    main()
