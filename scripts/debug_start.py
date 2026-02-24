#!/usr/bin/env python3
import argparse
import os
import subprocess
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


def update_cli_config(root: Path, gateway_cfg: Path) -> None:
    if not gateway_cfg.exists():
        return
    data = yaml.safe_load(gateway_cfg.read_text(encoding="utf-8")) or {}
    host = data.get("Host") or data.get("host")
    port = data.get("Port") or data.get("port")
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


def start_service(root: Path, log_dir: Path, bin_dir: Path, service: Dict) -> None:
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
    with log_path.open("ab") as log_file:
        proc = subprocess.Popen(
            [str(bin_path), "-f", str(config_path)],
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


def main() -> None:
    parser = argparse.ArgumentParser(description="Start debug deps and services")
    parser.add_argument("--manifest", default="scripts/debug_manifest.yaml", help="Path to manifest")
    parser.add_argument("--no-deps", action="store_true", help="Skip starting dependencies")
    parser.add_argument("--no-build", action="store_true", help="Skip building binaries")
    parser.add_argument("--deps-only", action="store_true", help="Only start dependencies")
    parser.add_argument("--services-only", action="store_true", help="Only start services")
    parser.add_argument("--only", default="", help="Comma-separated service list")
    args = parser.parse_args()

    manifest_path = Path(args.manifest)
    manifest = load_manifest(manifest_path)
    root = (manifest_path.parent / manifest.get("rootDir", ".")).resolve()

    log_dir = root / manifest.get("logDir", "logs")
    bin_dir = root / manifest.get("binDir", "logs/bin")
    log_dir.mkdir(parents=True, exist_ok=True)
    bin_dir.mkdir(parents=True, exist_ok=True)

    base_cmd = compose_base(manifest, root)
    env = compose_env(manifest)

    if not args.no_deps and not args.services_only:
        run(base_cmd + ["up", "-d"], env=env)
        wait_for_mysql(base_cmd, env)
        wait_for_redis(base_cmd, env)
        wait_for_kafka(base_cmd, env)
        wait_for_http(manifest["deps"]["etcd"]["healthURL"], timeout_s=60)
        wait_for_http(manifest["deps"]["elasticsearch"]["healthURL"], timeout_s=90)
        wait_for_http(manifest["deps"]["kibana"]["healthURL"], timeout_s=120)
        wait_for_http(manifest["deps"]["minio"]["healthURL"], timeout_s=60)

        init_mysql(base_cmd, env)
        for schema in manifest["deps"]["mysql"].get("schemas", []):
            import_schema(base_cmd, env, root / schema)

        project = manifest["deps"].get("projectName", "fuzoj")
        ensure_minio_buckets(project, manifest["deps"]["minio"])
        ensure_kafka_topics(base_cmd, env, root, manifest["deps"]["kafka"]["ensureTopicsScript"])

    if args.deps_only:
        return

    only_set = {name.strip() for name in args.only.split(",") if name.strip()}
    services = manifest.get("services", [])
    if not isinstance(services, list):
        raise SystemExit("manifest services must be a list")

    if not args.no_build:
        for svc in services:
            if only_set and svc["name"] not in only_set:
                continue
            build_service(root, bin_dir, svc)

    for svc in services:
        if only_set and svc["name"] not in only_set:
            continue
        start_service(root, log_dir, bin_dir, svc)

    gateway_config = None
    for svc in services:
        if svc.get("name") == "gateway":
            gateway_config = root / svc["config"]
            break
    if gateway_config is not None:
        update_cli_config(root, gateway_config)

    print("services started")
    print(f"logs: {log_dir}")


if __name__ == "__main__":
    main()
