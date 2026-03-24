#!/usr/bin/env python3
import argparse
import json
import re
import statistics
import subprocess
import sys
import time
from datetime import datetime, timezone
from pathlib import Path


def require(cond: bool, message: str) -> None:
    if not cond:
        raise RuntimeError(message)


def parse_steps(raw: str) -> list[int]:
    values = []
    for part in raw.split(","):
        text = part.strip()
        if not text:
            continue
        values.append(int(text))
    require(values, "concurrency-steps is empty")
    for value in values:
        require(value > 0, f"concurrency step must be positive: {value}")
    return values


def parse_metric(text: str, key: str) -> int | None:
    match = re.search(rf"^{re.escape(key)}=(\d+)$", text, flags=re.MULTILINE)
    if not match:
        return None
    return int(match.group(1))


def parse_metric_float(text: str, key: str) -> float | None:
    match = re.search(rf"^{re.escape(key)}=([0-9]+(?:\.[0-9]+)?)$", text, flags=re.MULTILINE)
    if not match:
        return None
    return float(match.group(1))


def build_smoke_cmd(args: argparse.Namespace, concurrent_users: int, seed: int) -> list[str]:
    cmd = [
        sys.executable,
        str(args.smoke_script),
        "--concurrent-users",
        str(concurrent_users),
        "--submit-workers",
        str(args.submit_workers),
        "--timeout",
        str(args.timeout),
        "--status-wait-mode",
        args.status_wait_mode,
        "--status-fetch-mode",
        args.status_fetch_mode,
        "--poll-interval",
        str(args.poll_interval),
        "--poll-times",
        str(args.poll_times),
        "--rank-poll-times",
        str(args.rank_poll_times),
        "--multi-submit-ratio",
        str(args.multi_submit_ratio),
        "--multi-submit-min",
        str(args.multi_submit_min),
        "--multi-submit-max",
        str(args.multi_submit_max),
        "--submit-gap-min-ms",
        str(args.submit_gap_min_ms),
        "--submit-gap-max-ms",
        str(args.submit_gap_max_ms),
        "--submit-seed",
        str(seed),
    ]
    if args.base:
        cmd.extend(["--base", args.base])
    if args.throughput_only:
        cmd.append("--throughput-only")
    return cmd


def run_once(args: argparse.Namespace, concurrent_users: int, round_index: int) -> dict:
    seed = args.seed + concurrent_users * 1000 + round_index
    cmd = build_smoke_cmd(args, concurrent_users, seed)
    started_at = time.time()
    proc = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=False)
    elapsed_s = max(0.001, time.time() - started_at)
    stdout = proc.stdout.decode("utf-8", errors="replace")
    stderr = proc.stderr.decode("utf-8", errors="replace")

    total_attempts = parse_metric(stdout, "total_planned_attempts")
    participant_count = parse_metric(stdout, "participant_count")
    submit_stage_elapsed_s = parse_metric_float(stdout, "submit_stage_elapsed_s")
    leaderboard_all_seen = "leaderboard_all_seen=True" in stdout
    ok = proc.returncode == 0
    qps_elapsed_s = submit_stage_elapsed_s if submit_stage_elapsed_s and submit_stage_elapsed_s > 0 else elapsed_s
    qps = (float(total_attempts) / qps_elapsed_s) if total_attempts is not None else 0.0

    return {
        "concurrent_users": concurrent_users,
        "round": round_index,
        "seed": seed,
        "ok": ok,
        "exit_code": proc.returncode,
        "elapsed_s": elapsed_s,
        "submit_stage_elapsed_s": submit_stage_elapsed_s,
        "total_planned_attempts": total_attempts,
        "participant_count": participant_count,
        "leaderboard_all_seen": leaderboard_all_seen,
        "qps": qps,
        "stdout": stdout,
        "stderr": stderr,
    }


def summarize(records: list[dict]) -> list[dict]:
    grouped: dict[int, list[dict]] = {}
    for item in records:
        grouped.setdefault(item["concurrent_users"], []).append(item)

    summary = []
    for concurrent_users in sorted(grouped.keys()):
        items = grouped[concurrent_users]
        qps_values = [x["qps"] for x in items if x["ok"] and x["total_planned_attempts"] is not None]
        elapsed_values = [x["elapsed_s"] for x in items]
        success_count = sum(1 for x in items if x["ok"])
        summary.append(
            {
                "concurrent_users": concurrent_users,
                "rounds": len(items),
                "success_count": success_count,
                "success_rate": success_count / len(items),
                "avg_elapsed_s": statistics.mean(elapsed_values),
                "p95_elapsed_s": statistics.quantiles(elapsed_values, n=20)[18] if len(elapsed_values) >= 2 else elapsed_values[0],
                "avg_qps": statistics.mean(qps_values) if qps_values else 0.0,
                "max_qps": max(qps_values) if qps_values else 0.0,
            }
        )
    return summary


def print_round_result(item: dict) -> None:
    status = "OK" if item["ok"] else "FAIL"
    attempts_text = str(item["total_planned_attempts"]) if item["total_planned_attempts"] is not None else "NA"
    submit_elapsed = item.get("submit_stage_elapsed_s")
    submit_elapsed_text = f"{submit_elapsed:.2f}s" if isinstance(submit_elapsed, float) else "NA"
    print(
        f"[users={item['concurrent_users']:>4} round={item['round']:>2}] "
        f"{status} elapsed={item['elapsed_s']:.2f}s submit_elapsed={submit_elapsed_text} "
        f"attempts={attempts_text} qps={item['qps']:.2f} "
        f"leaderboard_all_seen={item['leaderboard_all_seen']} seed={item['seed']}"
    )
    if not item["ok"]:
        stdout_tail = "\n".join(item["stdout"].splitlines()[-12:]).strip()
        stderr_tail = "\n".join(item["stderr"].splitlines()[-12:]).strip()
        if stdout_tail:
            print("  smoke_stdout_tail:")
            print(stdout_tail)
        if stderr_tail:
            print("  smoke_stderr_tail:")
            print(stderr_tail)


def print_summary(summary: list[dict]) -> None:
    print("\n== benchmark summary ==")
    print("users  rounds  success  avg_elapsed_s  p95_elapsed_s  avg_qps  max_qps")
    for item in summary:
        print(
            f"{item['concurrent_users']:>5}  "
            f"{item['rounds']:>6}  "
            f"{item['success_count']:>7}/{item['rounds']:<3}  "
            f"{item['avg_elapsed_s']:>13.2f}  "
            f"{item['p95_elapsed_s']:>13.2f}  "
            f"{item['avg_qps']:>7.2f}  "
            f"{item['max_qps']:>7.2f}"
        )


def main() -> int:
    parser = argparse.ArgumentParser(description="Benchmark QPS for tests/e2e_contest_rank_smoke.py")
    parser.add_argument("--base", default="", help="Gateway base URL, forwarded to smoke script")
    parser.add_argument("--smoke-script", default="tests/e2e_contest_rank_smoke.py", help="Path to e2e smoke script")
    parser.add_argument("--concurrency-steps", default="20,50,100", help="Comma-separated user counts")
    parser.add_argument("--rounds", type=int, default=3, help="Rounds per concurrency step")
    parser.add_argument("--submit-workers", type=int, default=50, help="Worker count forwarded to smoke script")
    parser.add_argument("--timeout", type=int, default=10, help="HTTP timeout seconds")
    parser.add_argument(
        "--status-wait-mode",
        choices=("sse", "poll", "auto"),
        default="sse",
        help="Forwarded to smoke script for submission status waiting",
    )
    parser.add_argument(
        "--status-fetch-mode",
        choices=("gateway", "status", "auto"),
        default="auto",
        help="Forwarded to smoke script for short polling fallback",
    )
    parser.add_argument("--poll-interval", type=float, default=1.0, help="Status poll interval seconds")
    parser.add_argument("--poll-times", type=int, default=60, help="Status poll times")
    parser.add_argument("--rank-poll-times", type=int, default=90, help="Leaderboard poll times")
    parser.add_argument("--multi-submit-ratio", type=float, default=0.4, help="Forwarded to smoke script")
    parser.add_argument("--multi-submit-min", type=int, default=2, help="Forwarded to smoke script")
    parser.add_argument("--multi-submit-max", type=int, default=4, help="Forwarded to smoke script")
    parser.add_argument("--submit-gap-min-ms", type=int, default=150, help="Forwarded to smoke script")
    parser.add_argument("--submit-gap-max-ms", type=int, default=1200, help="Forwarded to smoke script")
    parser.add_argument("--seed", type=int, default=20260314, help="Base random seed for deterministic rounds")
    parser.add_argument(
        "--throughput-only",
        action="store_true",
        help="Forward to smoke script and measure submit throughput without status/leaderboard waits",
    )
    parser.add_argument("--stop-on-fail", action="store_true", help="Stop immediately when a round fails")
    parser.add_argument("--output-json", default="", help="Write raw benchmark report to json file")
    parser.add_argument("--save-logs-dir", default="", help="Directory to save per-round stdout/stderr logs")
    args = parser.parse_args()

    require(args.rounds > 0, "rounds must be positive")
    steps = parse_steps(args.concurrency_steps)
    smoke_script = Path(args.smoke_script).resolve()
    require(smoke_script.exists(), f"smoke script not found: {smoke_script}")
    args.smoke_script = smoke_script

    records = []
    started_iso = datetime.now(timezone.utc).isoformat()
    logs_dir = Path(args.save_logs_dir).resolve() if args.save_logs_dir else None
    if logs_dir:
        logs_dir.mkdir(parents=True, exist_ok=True)

    print(f"benchmark_start={started_iso}")
    print(f"smoke_script={smoke_script}")
    print(f"steps={steps} rounds={args.rounds}")
    for users in steps:
        for round_index in range(1, args.rounds + 1):
            item = run_once(args, users, round_index)
            records.append(item)
            print_round_result(item)

            if logs_dir:
                base_name = f"users_{users}_round_{round_index}"
                (logs_dir / f"{base_name}.stdout.log").write_text(item["stdout"], encoding="utf-8")
                (logs_dir / f"{base_name}.stderr.log").write_text(item["stderr"], encoding="utf-8")

            if args.stop_on_fail and not item["ok"]:
                print("stop on first failure enabled, exiting benchmark early")
                summary = summarize(records)
                print_summary(summary)
                return 1

    summary = summarize(records)
    print_summary(summary)

    if args.output_json:
        output_path = Path(args.output_json).resolve()
        output_path.parent.mkdir(parents=True, exist_ok=True)
        payload = {
            "started_at_utc": started_iso,
            "smoke_script": str(smoke_script),
            "steps": steps,
            "rounds": args.rounds,
            "records": records,
            "summary": summary,
        }
        output_path.write_text(json.dumps(payload, ensure_ascii=True, indent=2), encoding="utf-8")
        print(f"json_report={output_path}")

    return 0 if all(item["ok"] for item in records) else 1


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception as exc:
        print(f"error: {exc}")
        sys.exit(1)
