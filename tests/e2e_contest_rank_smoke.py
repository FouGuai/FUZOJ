#!/usr/bin/env python3
import argparse
import hashlib
import json
import subprocess
import sys
import tempfile
import time
import uuid
from datetime import datetime, timedelta, timezone
from pathlib import Path

import requests
import yaml


def load_base_url(repo_root: Path, override: str) -> str:
    if override:
        return override.rstrip("/")
    config_path = repo_root / "configs/cli.yaml"
    if not config_path.exists():
        return "http://127.0.0.1:8080"
    data = yaml.safe_load(config_path.read_text(encoding="utf-8")) or {}
    base_url = data.get("baseURL") or "http://127.0.0.1:8080"
    return str(base_url).rstrip("/")


def sha256_file(path: Path) -> str:
    hasher = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            hasher.update(chunk)
    return hasher.hexdigest()


def write_json(path: Path, payload: dict) -> None:
    path.write_text(json.dumps(payload, ensure_ascii=True, indent=2), encoding="utf-8")


def run_cmd(args: list[str]) -> None:
    proc = subprocess.run(args, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    if proc.returncode != 0:
        raise RuntimeError(f"command failed: {' '.join(args)}\nstdout:\n{proc.stdout}\nstderr:\n{proc.stderr}")


def pretty_body(resp: requests.Response) -> str:
    try:
        data = resp.json()
    except ValueError:
        return resp.text
    return json.dumps(data, ensure_ascii=True, indent=2)


def pick(data: dict, *keys):
    cur = data
    for key in keys:
        if not isinstance(cur, dict):
            return None
        cur = cur.get(key)
    return cur


def require(cond: bool, message: str) -> None:
    if not cond:
        raise RuntimeError(message)


def request_json(session: requests.Session, method: str, url: str, *, headers=None, payload=None, timeout=10) -> dict:
    start = time.time()
    resp = session.request(method, url, headers=headers, json=payload, timeout=timeout)
    elapsed = time.time() - start
    print(f"{method} {url} -> {resp.status_code} ({elapsed:.2f}s)")
    print(pretty_body(resp))
    if resp.status_code < 200 or resp.status_code >= 300:
        raise RuntimeError(f"request failed: {method} {url} ({resp.status_code})")
    try:
        return resp.json()
    except ValueError as exc:
        raise RuntimeError("response is not json") from exc


def upload_file(url: str, path: Path, timeout: int) -> str:
    with path.open("rb") as f:
        resp = requests.put(url, data=f, timeout=timeout)
    print(f"PUT {url} -> {resp.status_code}")
    print(resp.text)
    if resp.status_code < 200 or resp.status_code >= 300:
        raise RuntimeError(f"upload failed: {resp.status_code}")
    etag = resp.headers.get("ETag", "")
    return etag.strip('"')


def to_rfc3339(ts: datetime) -> str:
    return ts.astimezone(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def main() -> int:
    parser = argparse.ArgumentParser(description="Run contest full-path e2e with rank validation.")
    parser.add_argument("--base", default="", help="Base URL for gateway")
    parser.add_argument("--timeout", type=int, default=10, help="HTTP timeout in seconds")
    parser.add_argument("--poll-interval", type=float, default=1.0, help="Polling interval in seconds")
    parser.add_argument("--poll-times", type=int, default=60, help="Polling attempts for submit status")
    parser.add_argument("--rank-poll-times", type=int, default=90, help="Polling attempts for leaderboard")
    args = parser.parse_args()

    repo_root = Path(__file__).resolve().parents[1]
    base_url = load_base_url(repo_root, args.base)
    session = requests.Session()
    session.headers.update({"Content-Type": "application/json"})

    username = f"contest_e2e_{uuid.uuid4().hex[:8]}"
    password = f"Demo!{uuid.uuid4().hex[:8]}A1"

    print("== register ==")
    try:
        request_json(
            session,
            "POST",
            f"{base_url}/api/v1/user/register",
            payload={"username": username, "password": password},
            timeout=args.timeout,
        )
    except RuntimeError as err:
        print(f"register failed, continue to login: {err}")

    print("== login ==")
    login_resp = request_json(
        session,
        "POST",
        f"{base_url}/api/v1/user/login",
        payload={"username": username, "password": password},
        timeout=args.timeout,
    )
    access_token = pick(login_resp, "data", "access_token") or login_resp.get("access_token")
    user_id = pick(login_resp, "data", "user", "id") or pick(login_resp, "user", "id")
    require(access_token, "access_token not found in login response")
    require(user_id, "user_id not found in login response")
    session.headers.update({"Authorization": f"Bearer {access_token}"})

    print("== problem create ==")
    problem_resp = request_json(
        session,
        "POST",
        f"{base_url}/api/v1/problems",
        payload={"title": "Contest Two Sum (E2E)", "owner_id": int(user_id)},
        timeout=args.timeout,
    )
    problem_id = pick(problem_resp, "data", "id") or pick(problem_resp, "data", "problem_id")
    require(problem_id, "problem_id not found in create response")

    version = 1
    statement = "# Contest Two Sum\nGiven nums, return a+b.\n"

    print("== build data pack ==")
    temp_root = Path(tempfile.mkdtemp(prefix="fuzoj-contest-e2e-"))
    pack_root = temp_root / "pack"
    tests_dir = pack_root / "tests"
    tests_dir.mkdir(parents=True, exist_ok=True)
    (tests_dir / "1.in").write_text("1 2\n", encoding="utf-8")
    (tests_dir / "1.out").write_text("3\n", encoding="utf-8")
    (tests_dir / "2.in").write_text("10 5\n", encoding="utf-8")
    (tests_dir / "2.out").write_text("15\n", encoding="utf-8")
    (tests_dir / "3.in").write_text("7 8\n", encoding="utf-8")
    (tests_dir / "3.out").write_text("15\n", encoding="utf-8")

    def build_pack(ver: int) -> tuple[Path, Path, Path]:
        manifest = {
            "problemId": int(problem_id),
            "version": int(ver),
            "ioConfig": {"mode": "stdio"},
            "tests": [
                {"testId": "1", "inputPath": "tests/1.in", "answerPath": "tests/1.out", "score": 40, "subtaskId": ""},
                {"testId": "2", "inputPath": "tests/2.in", "answerPath": "tests/2.out", "score": 30, "subtaskId": ""},
                {"testId": "3", "inputPath": "tests/3.in", "answerPath": "tests/3.out", "score": 30, "subtaskId": ""},
            ],
            "subtasks": [],
            "hash": {"manifestHash": "", "dataPackHash": ""},
        }
        config = {
            "problemId": int(problem_id),
            "version": int(ver),
            "title": "Contest Two Sum (E2E)",
            "defaultLimits": {
                "timeMs": 1000,
                "wallTimeMs": 2000,
                "memoryMB": 256,
                "stackMB": 64,
                "outputMB": 64,
                "processes": 64,
            },
            "languageLimits": [],
        }
        manifest_path = pack_root / "manifest.json"
        config_path = pack_root / "config.json"
        write_json(manifest_path, manifest)
        write_json(config_path, config)
        tar_path = temp_root / "data-pack.tar.zst"
        if tar_path.exists():
            tar_path.unlink()
        run_cmd(["tar", "--zstd", "-cf", str(tar_path), "-C", str(pack_root), "."])
        return tar_path, manifest_path, config_path

    tar_path, manifest_path, config_path = build_pack(version)
    data_pack_hash = sha256_file(tar_path)
    manifest_hash = sha256_file(manifest_path)
    expected_size = tar_path.stat().st_size

    def upload_prepare(expected_size_bytes: int, expected_sha256: str) -> dict:
        idem_key = str(uuid.uuid4())
        headers = {"Idempotency-Key": idem_key}
        auth_header = session.headers.get("Authorization")
        if auth_header:
            headers["Authorization"] = auth_header
        headers["Content-Type"] = "application/json"
        return request_json(
            session,
            "POST",
            f"{base_url}/api/v1/problems/{problem_id}/data-pack/uploads:prepare",
            headers=headers,
            payload={
                "expected_size_bytes": expected_size_bytes,
                "expected_sha256": expected_sha256,
                "content_type": "application/zstd",
                "created_by": int(user_id),
                "client_type": "e2e_contest_rank",
                "upload_strategy": "multipart",
            },
            timeout=args.timeout,
        )

    print("== upload prepare ==")
    prepare_resp = upload_prepare(expected_size, data_pack_hash)
    upload_id = pick(prepare_resp, "data", "upload_id")
    resp_version = pick(prepare_resp, "data", "version") or version
    require(upload_id, "upload_id not found in prepare response")

    if int(resp_version) != version:
        version = int(resp_version)
        tar_path, manifest_path, config_path = build_pack(version)
        data_pack_hash = sha256_file(tar_path)
        manifest_hash = sha256_file(manifest_path)
        expected_size = tar_path.stat().st_size
        print("== upload prepare (version adjusted) ==")
        prepare_resp = upload_prepare(expected_size, data_pack_hash)
        upload_id = pick(prepare_resp, "data", "upload_id")
        require(upload_id, "upload_id not found after version adjustment")

    print("== statement update ==")
    request_json(
        session,
        "PUT",
        f"{base_url}/api/v1/problems/{problem_id}/versions/{version}/statement",
        payload={"statement_md": statement},
        timeout=args.timeout,
    )

    print("== upload sign ==")
    sign_resp = request_json(
        session,
        "POST",
        f"{base_url}/api/v1/problems/{problem_id}/data-pack/uploads/{upload_id}/sign",
        payload={"part_numbers": [1]},
        timeout=args.timeout,
    )
    urls = pick(sign_resp, "data", "urls") or {}
    signed_url = urls.get("1") if isinstance(urls, dict) else None
    require(signed_url, "signed url not found in sign response")

    print("== upload part ==")
    etag = upload_file(signed_url, tar_path, args.timeout)

    print("== upload complete ==")
    manifest_json = json.loads(manifest_path.read_text(encoding="utf-8"))
    config_json = json.loads(config_path.read_text(encoding="utf-8"))
    request_json(
        session,
        "POST",
        f"{base_url}/api/v1/problems/{problem_id}/data-pack/uploads/{upload_id}/complete",
        payload={
            "parts": [{"part_number": 1, "etag": etag}],
            "manifest_json": json.dumps(manifest_json, ensure_ascii=True),
            "config_json": json.dumps(config_json, ensure_ascii=True),
            "manifest_hash": manifest_hash,
            "data_pack_hash": data_pack_hash,
        },
        timeout=args.timeout,
    )

    print("== problem publish ==")
    request_json(session, "POST", f"{base_url}/api/v1/problems/{problem_id}/versions/{version}/publish", timeout=args.timeout)

    now = datetime.now(timezone.utc)
    contest_start = to_rfc3339(now - timedelta(minutes=1))
    contest_end = to_rfc3339(now + timedelta(minutes=30))

    print("== contest create ==")
    contest_resp = request_json(
        session,
        "POST",
        f"{base_url}/api/v1/contests",
        payload={
            "title": "Contest E2E Smoke",
            "description": "contest rank smoke",
            "visibility": "public",
            "owner_id": int(user_id),
            "org_id": 0,
            "start_at": contest_start,
            "end_at": contest_end,
            "rule": {"rule_type": "icpc", "penalty_minutes": 20, "freeze_minutes_before_end": 0},
        },
        timeout=args.timeout,
    )
    contest_id = pick(contest_resp, "data", "contest_id")
    require(contest_id, "contest_id not found")

    print("== contest problem add ==")
    request_json(
        session,
        "POST",
        f"{base_url}/api/v1/contests/{contest_id}/problems",
        payload={"problem_id": int(problem_id), "order": 1, "score": 100, "visible": True, "version": int(version)},
        timeout=args.timeout,
    )

    print("== contest publish ==")
    request_json(session, "POST", f"{base_url}/api/v1/contests/{contest_id}/publish", timeout=args.timeout)

    print("== contest register ==")
    request_json(
        session,
        "POST",
        f"{base_url}/api/v1/contests/{contest_id}/register",
        payload={"user_id": int(user_id), "team_id": "", "invite_code": ""},
        timeout=args.timeout,
    )

    source_path = repo_root / "tests/main.cpp"
    require(source_path.exists(), "tests/main.cpp not found")
    source_code = source_path.read_text(encoding="utf-8")

    print("== contest submit create ==")
    submit_resp = request_json(
        session,
        "POST",
        f"{base_url}/api/v1/submissions",
        headers={"Idempotency-Key": str(uuid.uuid4())},
        payload={
            "problem_id": int(problem_id),
            "user_id": int(user_id),
            "language_id": "cpp",
            "source_code": source_code,
            "contest_id": contest_id,
            "scene": "contest",
            "extra_compile_flags": [],
        },
        timeout=args.timeout,
    )
    submission_id = pick(submit_resp, "data", "submission_id") or pick(submit_resp, "submission_id")
    require(submission_id, "submission_id not found")

    print("== contest submit status ==")
    final_status = ""
    for _ in range(args.poll_times):
        status_resp = request_json(session, "GET", f"{base_url}/api/v1/submissions/{submission_id}", timeout=args.timeout)
        final_status = pick(status_resp, "data", "status") or status_resp.get("status") or ""
        if final_status.lower() in {"finished", "failed"}:
            break
        time.sleep(args.poll_interval)
    require(final_status != "", "final status is empty")

    print("== leaderboard poll ==")
    rank_found = False
    rank_value = None
    for _ in range(args.rank_poll_times):
        lb_resp = request_json(
            session,
            "GET",
            f"{base_url}/api/v1/contests/{contest_id}/leaderboard?page=1&page_size=50&mode=live",
            timeout=args.timeout,
        )
        items = pick(lb_resp, "data", "items") or []
        for item in items:
            member_id = str(item.get("member_id", ""))
            if member_id == str(user_id):
                rank_found = True
                rank_value = item.get("rank")
                break
        if rank_found:
            break
        time.sleep(args.poll_interval)
    require(rank_found, "member not found in leaderboard within timeout")

    print("== summary ==")
    print(f"user_id={user_id}")
    print(f"problem_id={problem_id}")
    print(f"contest_id={contest_id}")
    print(f"submission_id={submission_id}")
    print(f"final_status={final_status}")
    print(f"rank_seen={rank_found}")
    print(f"rank={rank_value}")
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception as exc:
        print(f"error: {exc}")
        sys.exit(1)
