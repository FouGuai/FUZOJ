#!/usr/bin/env python3
import argparse
import hashlib
import json
import random
import re
import socket
import subprocess
import sys
import tempfile
import time
import uuid
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime, timedelta, timezone
from pathlib import Path
from urllib.parse import urlparse

import requests
import yaml


def is_http_reachable(base_url: str, timeout_s: float = 1.0) -> bool:
    try:
        parsed = requests.utils.urlparse(base_url)
        host = parsed.hostname
        port = parsed.port
        if not host or not port:
            return False
        with socket.create_connection((host, port), timeout=timeout_s):
            return True
    except OSError:
        return False


def load_base_url(repo_root: Path, override: str) -> str:
    default_url = "http://127.0.0.1:8080"
    if override:
        return override.rstrip("/")
    config_path = repo_root / "configs/cli.yaml"
    if not config_path.exists():
        return default_url
    data = yaml.safe_load(config_path.read_text(encoding="utf-8")) or {}
    base_url = str(data.get("baseURL") or default_url).rstrip("/")
    if is_http_reachable(base_url):
        return base_url
    return default_url


def discover_service_base_url(repo_root: Path, service_name: str) -> str:
    try:
        proc = subprocess.run(
            ["python3", "scripts/status_services.py"],
            cwd=str(repo_root),
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            check=False,
        )
    except OSError:
        return ""
    if proc.returncode != 0:
        return ""
    pattern = re.compile(rf"^{re.escape(service_name)}:.*rest ([^,\s\)]+)")
    for line in proc.stdout.splitlines():
        match = pattern.search(line.strip())
        if not match:
            continue
        host_port = match.group(1)
        if ":" not in host_port:
            continue
        host, port = host_port.rsplit(":", 1)
        host = "127.0.0.1" if host in ("0.0.0.0", "::") else host
        return f"http://{host}:{port}"
    return ""


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


def try_request_json(session: requests.Session, method: str, url: str, *, headers=None, payload=None, timeout=10):
    start = time.time()
    resp = session.request(method, url, headers=headers, json=payload, timeout=timeout)
    elapsed = time.time() - start
    print(f"{method} {url} -> {resp.status_code} ({elapsed:.2f}s)")
    print(pretty_body(resp))
    if 200 <= resp.status_code < 300:
        try:
            return True, resp.json()
        except ValueError as exc:
            raise RuntimeError("response is not json") from exc
    return False, {"status": resp.status_code, "body": pretty_body(resp)}


def replace_base(url: str, new_base: str) -> str:
    old = urlparse(url)
    new = urlparse(new_base)
    return f"{new.scheme}://{new.netloc}{old.path}" + (f"?{old.query}" if old.query else "")


def create_authenticated_user(base_url: str, timeout: int, username_prefix: str) -> tuple[requests.Session, int, str]:
    session = requests.Session()
    session.headers.update({"Content-Type": "application/json"})
    username = f"{username_prefix}_{uuid.uuid4().hex[:8]}"
    password = f"Demo!{uuid.uuid4().hex[:8]}A1"

    try:
        request_json(
            session,
            "POST",
            f"{base_url}/api/v1/user/register",
            payload={"username": username, "password": password},
            timeout=timeout,
        )
    except RuntimeError as err:
        print(f"register failed, continue to login: {err}")

    login_resp = request_json(
        session,
        "POST",
        f"{base_url}/api/v1/user/login",
        payload={"username": username, "password": password},
        timeout=timeout,
    )
    access_token = pick(login_resp, "data", "access_token") or login_resp.get("access_token")
    user_id = pick(login_resp, "data", "user", "id") or pick(login_resp, "user", "id")
    require(access_token, "access_token not found in login response")
    require(user_id, "user_id not found in login response")
    session.headers.update({"Authorization": f"Bearer {access_token}"})
    return session, int(user_id), username


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
    parser.add_argument("--concurrent-users", type=int, default=5, help="Total contest users for concurrent submit")
    parser.add_argument("--submit-workers", type=int, default=5, help="Worker count for concurrent submit/poll")
    parser.add_argument("--multi-submit-ratio", type=float, default=0.4, help="Ratio of users with multi-submit attempts")
    parser.add_argument("--multi-submit-min", type=int, default=2, help="Minimum attempts for multi-submit users")
    parser.add_argument("--multi-submit-max", type=int, default=4, help="Maximum attempts for multi-submit users")
    parser.add_argument("--submit-gap-min-ms", type=int, default=150, help="Minimum delay between user attempts in ms")
    parser.add_argument("--submit-gap-max-ms", type=int, default=1200, help="Maximum delay between user attempts in ms")
    parser.add_argument("--submit-seed", type=int, default=0, help="Random seed for submit behavior (0 means time-based)")
    parser.add_argument(
        "--throughput-only",
        action="store_true",
        help="Measure submit throughput only: skip status wait and leaderboard convergence checks",
    )
    args = parser.parse_args()
    require(0.0 <= args.multi_submit_ratio <= 1.0, "multi-submit-ratio must be between 0 and 1")
    require(args.multi_submit_min >= 2, "multi-submit-min must be at least 2")
    require(args.multi_submit_max >= args.multi_submit_min, "multi-submit-max must be >= multi-submit-min")
    require(args.submit_gap_min_ms >= 0, "submit-gap-min-ms must be non-negative")
    require(args.submit_gap_max_ms >= args.submit_gap_min_ms, "submit-gap-max-ms must be >= submit-gap-min-ms")
    behavior_seed = args.submit_seed if args.submit_seed != 0 else time.time_ns()
    rng = random.Random(behavior_seed)
    print(f"submit behavior seed={behavior_seed}")

    repo_root = Path(__file__).resolve().parents[1]
    base_url = load_base_url(repo_root, args.base)
    if not is_http_reachable(base_url):
        gateway_base_url = discover_service_base_url(repo_root, "gateway")
        if gateway_base_url:
            base_url = gateway_base_url
            print(f"gateway direct base detected: {base_url}")
    contest_base_url = discover_service_base_url(repo_root, "contest-service")
    status_base_url = discover_service_base_url(repo_root, "status-service")
    rank_base_url = discover_service_base_url(repo_root, "rank-service")
    if contest_base_url:
        print(f"contest direct base detected: {contest_base_url}")
    if status_base_url:
        print(f"status direct base detected: {status_base_url}")
    if rank_base_url:
        print(f"rank direct base detected: {rank_base_url}")
    print("== register ==")
    print("== login ==")
    session, user_id, _ = create_authenticated_user(base_url, args.timeout, "contest_e2e_owner")

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
    contest_create_payloads = [
        {
            "title": "Contest E2E Smoke",
            "description": "contest rank smoke",
            "visibility": "public",
            "owner_id": int(user_id),
            "org_id": 0,
            "start_at": contest_start,
            "end_at": contest_end,
            "rule": {
                "rule_type": "icpc",
                "penalty_minutes": 20,
                "penalty_formula": "",
                "penalty_cap_minutes": 0,
                "freeze_minutes_before_end": 0,
                "allow_hack": False,
                "hack_reward": 0,
                "hack_penalty": 0,
                "max_submissions_per_problem": 0,
                "score_mode": "sum",
                "publish_solutions_after_end": False,
                "virtual_participation_enabled": False,
            },
        },
        {
            "title": "Contest E2E Smoke",
            "start_at": contest_start,
            "end_at": contest_end,
        },
        {
            "title": "Contest E2E Smoke",
            "description": "contest rank smoke",
            "visibility": "public",
            "ownerId": int(user_id),
            "orgId": 0,
            "startAt": contest_start,
            "endAt": contest_end,
            "rule": {
                "ruleType": "icpc",
                "penaltyMinutes": 20,
                "penaltyFormula": "",
                "penaltyCapMinutes": 0,
                "freezeMinutesBeforeEnd": 0,
                "allowHack": False,
                "hackReward": 0,
                "hackPenalty": 0,
                "maxSubmissionsPerProblem": 0,
                "scoreMode": "sum",
                "publishSolutionsAfterEnd": False,
                "virtualParticipationEnabled": False,
            },
        },
        {
            "title": "Contest E2E Smoke",
            "startAt": contest_start,
            "endAt": contest_end,
        },
    ]
    contest_resp = None
    for idx, payload in enumerate(contest_create_payloads, start=1):
        ok, result = try_request_json(
            session,
            "POST",
            f"{base_url}/api/v1/contests",
            payload=payload,
            timeout=args.timeout,
        )
        if ok:
            contest_resp = result
            break
        print(f"contest create attempt {idx} failed, trying next payload")
    if contest_resp is None:
        raise RuntimeError("contest create failed for all payload variants")

    contest_id = pick(contest_resp, "data", "contest_id")
    require(contest_id, "contest_id not found")

    print("== contest problem add ==")
    contest_problem_url = f"{base_url}/api/v1/contests/{contest_id}/problems"
    ok, _ = try_request_json(
        session,
        "POST",
        contest_problem_url,
        payload={"problem_id": int(problem_id), "order": 1, "score": 100, "visible": True, "version": int(version)},
        timeout=args.timeout,
    )
    if not ok:
        if contest_base_url:
            request_json(
                session,
                "POST",
                replace_base(contest_problem_url, contest_base_url),
                payload={"problem_id": int(problem_id), "order": 1, "score": 100, "visible": True, "version": int(version)},
                timeout=args.timeout,
            )
        else:
            raise RuntimeError(f"request failed: POST {contest_problem_url}")

    print("== contest publish ==")
    contest_publish_url = f"{base_url}/api/v1/contests/{contest_id}/publish"
    ok, _ = try_request_json(session, "POST", contest_publish_url, timeout=args.timeout)
    if not ok:
        if contest_base_url:
            request_json(session, "POST", replace_base(contest_publish_url, contest_base_url), timeout=args.timeout)
        else:
            raise RuntimeError(f"request failed: POST {contest_publish_url}")

    print("== contest register ==")
    contest_register_url = f"{base_url}/api/v1/contests/{contest_id}/register"
    register_payload = {"user_id": int(user_id), "team_id": "", "invite_code": ""}
    ok, _ = try_request_json(
        session,
        "POST",
        contest_register_url,
        payload=register_payload,
        timeout=args.timeout,
    )
    if not ok:
        if contest_base_url:
            request_json(
                session,
                "POST",
                replace_base(contest_register_url, contest_base_url),
                payload=register_payload,
                timeout=args.timeout,
            )
        else:
            raise RuntimeError(f"request failed: POST {contest_register_url}")

    source_path = repo_root / "tests/main.cpp"
    require(source_path.exists(), "tests/main.cpp not found")
    source_code = source_path.read_text(encoding="utf-8")
    wrong_source_code = """
#include <bits/stdc++.h>
using namespace std;
int main() {
    long long a = 0, b = 0;
    if (!(cin >> a >> b)) return 0;
    cout << (a - b) << "\\n";
    return 0;
}
""".strip()

    print("== build concurrent users ==")
    require(args.concurrent_users > 0, "concurrent-users must be positive")
    participants = [{"session": session, "user_id": int(user_id)}]
    remaining_users = args.concurrent_users - 1
    if remaining_users > 0:
        create_workers = max(1, min(args.submit_workers, remaining_users))
        with ThreadPoolExecutor(max_workers=create_workers) as executor:
            futures = [
                executor.submit(create_authenticated_user, base_url, args.timeout, "contest_e2e_member")
                for _ in range(remaining_users)
            ]
            for future in as_completed(futures):
                u_session, u_id, _ = future.result()
                participants.append({"session": u_session, "user_id": int(u_id)})

    print("== contest register all users ==")
    for participant in participants:
        participant_session = participant["session"]
        participant_id = participant["user_id"]
        register_payload = {"user_id": int(participant_id), "team_id": "", "invite_code": ""}
        ok, _ = try_request_json(
            participant_session,
            "POST",
            contest_register_url,
            payload=register_payload,
            timeout=args.timeout,
        )
        if not ok:
            if contest_base_url:
                request_json(
                    participant_session,
                    "POST",
                    replace_base(contest_register_url, contest_base_url),
                    payload=register_payload,
                    timeout=args.timeout,
                )
            else:
                raise RuntimeError(f"request failed: POST {contest_register_url}")

    print("== build submit behavior ==")
    total_users = len(participants)
    multi_user_count = 0
    if total_users > 0 and args.multi_submit_ratio > 0:
        multi_user_count = max(1, int(round(total_users * args.multi_submit_ratio)))
        multi_user_count = min(total_users, multi_user_count)
    multi_user_indexes = set(rng.sample(range(total_users), multi_user_count)) if multi_user_count > 0 else set()
    total_attempts = 0
    for idx, participant in enumerate(participants):
        submit_count = 1
        if idx in multi_user_indexes:
            submit_count = rng.randint(args.multi_submit_min, args.multi_submit_max)
        participant["planned_submit_count"] = submit_count
        participant["attempt_gaps_ms"] = [rng.randint(args.submit_gap_min_ms, args.submit_gap_max_ms) for _ in range(submit_count - 1)]
        total_attempts += submit_count
    print(f"participants={total_users} multi_submit_users={multi_user_count} total_planned_attempts={total_attempts}")

    def submit_once(participant: dict, code_text: str) -> dict:
        participant_session = participant["session"]
        participant_id = participant["user_id"]
        submit_resp = request_json(
            participant_session,
            "POST",
            f"{base_url}/api/v1/submissions",
            headers={"Idempotency-Key": str(uuid.uuid4())},
            payload={
                "problem_id": int(problem_id),
                "user_id": int(participant_id),
                "language_id": "cpp",
                "source_code": code_text,
                "contest_id": contest_id,
                "scene": "contest",
                "extra_compile_flags": [],
            },
            timeout=args.timeout,
        )
        submission_id = pick(submit_resp, "data", "submission_id") or pick(submit_resp, "submission_id")
        require(submission_id, f"submission_id not found for user={participant_id}")
        return {"user_id": participant_id, "submission_id": submission_id}

    def wait_submission_final(participant_session: requests.Session, participant_id: int, submission_id: str) -> str:
        user_status_urls = [f"{base_url}/api/v1/status/submissions/{submission_id}"]
        if status_base_url:
            user_status_urls.append(f"{status_base_url}/api/v1/status/submissions/{submission_id}")
        final_status = ""
        for _ in range(args.poll_times):
            status_resp = None
            for url in user_status_urls:
                ok, result = try_request_json(participant_session, "GET", url, timeout=args.timeout)
                if ok:
                    status_resp = result
                    break
            if status_resp is None:
                time.sleep(args.poll_interval)
                continue
            final_status = pick(status_resp, "data", "status") or status_resp.get("status") or ""
            if final_status.lower() in {"finished", "failed"}:
                break
            time.sleep(args.poll_interval)
        require(final_status != "", f"final status is empty for user={participant_id}")
        return final_status

    def submit_user_session(participant: dict) -> dict:
        submit_count = int(participant.get("planned_submit_count", 1))
        gaps_ms = participant.get("attempt_gaps_ms", [])
        attempts = []
        for attempt_idx in range(submit_count):
            is_final_attempt = attempt_idx == submit_count - 1
            code_text = source_code if is_final_attempt else wrong_source_code
            submit_result = submit_once(participant, code_text)
            if args.throughput_only:
                final_status = "submitted"
            else:
                final_status = wait_submission_final(
                    participant["session"],
                    int(participant["user_id"]),
                    str(submit_result["submission_id"]),
                )
            result = {
                "user_id": submit_result["user_id"],
                "submission_id": submit_result["submission_id"],
                "final_status": final_status,
            }
            attempts.append(result)
            if attempt_idx < submit_count - 1:
                gap_ms = int(gaps_ms[attempt_idx]) if attempt_idx < len(gaps_ms) else args.submit_gap_min_ms
                if gap_ms > 0:
                    time.sleep(gap_ms / 1000.0)
        last_attempt = attempts[-1]
        return {
            "user_id": participant["user_id"],
            "planned_submit_count": submit_count,
            "attempts": attempts,
            "last_submission_id": last_attempt["submission_id"],
            "last_final_status": last_attempt["final_status"],
        }

    print("== concurrent contest submit & status poll ==")
    submission_results = []
    max_workers = max(1, min(args.submit_workers, len(participants)))
    submit_stage_started_at = time.time()
    with ThreadPoolExecutor(max_workers=max_workers) as executor:
        futures = [executor.submit(submit_user_session, participant) for participant in participants]
        for future in as_completed(futures):
            submission_results.append(future.result())
    submit_stage_elapsed_s = max(0.001, time.time() - submit_stage_started_at)

    require(len(submission_results) == len(participants), "not all concurrent submissions completed")
    mismatched_attempts = [
        res
        for res in submission_results
        if int(res.get("planned_submit_count", 0)) != len(res.get("attempts", []))
    ]
    require(not mismatched_attempts, f"some users did not finish planned attempts: {mismatched_attempts}")
    if not args.throughput_only:
        failed_status = [res for res in submission_results if str(res["last_final_status"]).lower() != "finished"]
        require(not failed_status, f"some submissions did not finish successfully: {failed_status}")

    rank_found_all = False
    if args.throughput_only:
        print("== leaderboard poll skipped (throughput-only) ==")
    else:
        print("== leaderboard poll (all users) ==")
        page_size = max(50, len(participants) + 5)
        leaderboard_urls = [f"{base_url}/api/v1/contests/{contest_id}/leaderboard?page=1&page_size={page_size}&mode=live"]
        if rank_base_url:
            leaderboard_urls.append(
                f"{rank_base_url}/api/v1/contests/{contest_id}/leaderboard?page=1&page_size={page_size}&mode=live"
            )
        expected_member_ids = {str(participant["user_id"]) for participant in participants}
        seen_member_ids = set()
        for _ in range(args.rank_poll_times):
            lb_resp = None
            for lb_url in leaderboard_urls:
                ok, result = try_request_json(session, "GET", lb_url, timeout=args.timeout)
                if ok:
                    lb_resp = result
                    break
            if lb_resp is None:
                time.sleep(args.poll_interval)
                continue
            items = pick(lb_resp, "data", "items") or []
            seen_member_ids = {str(item.get("member_id", "")) for item in items if str(item.get("member_id", ""))}
            if expected_member_ids.issubset(seen_member_ids):
                rank_found_all = True
                break
            time.sleep(args.poll_interval)
        require(rank_found_all, f"not all members found in leaderboard, expected={expected_member_ids}, seen={seen_member_ids}")

    print("== summary ==")
    print(f"owner_user_id={user_id}")
    print(f"participant_count={len(participants)}")
    print(f"multi_submit_user_count={multi_user_count}")
    print(f"total_planned_attempts={total_attempts}")
    print(f"submit_stage_elapsed_s={submit_stage_elapsed_s:.6f}")
    print(f"throughput_only={args.throughput_only}")
    print(f"problem_id={problem_id}")
    print(f"contest_id={contest_id}")
    for result in sorted(submission_results, key=lambda item: item["user_id"]):
        attempt_summaries = ",".join(
            f"{idx + 1}:{attempt['submission_id']}:{attempt['final_status']}"
            for idx, attempt in enumerate(result["attempts"])
        )
        print(f"submission user_id={result['user_id']} attempts={attempt_summaries}")
    print(f"leaderboard_all_seen={rank_found_all}")
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception as exc:
        print(f"error: {exc}")
        sys.exit(1)
