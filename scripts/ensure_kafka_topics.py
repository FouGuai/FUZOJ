#!/usr/bin/env python3
import sys
from pathlib import Path

try:
    import yaml
except ImportError as exc:
    raise SystemExit("PyYAML is required to parse configs; please install pyyaml") from exc


def load_yaml(path: Path):
    try:
        data = yaml.safe_load(path.read_text(encoding="utf-8"))
    except FileNotFoundError:
        return None
    if not isinstance(data, dict):
        return None
    return data


def append_topic(topics, seen, value):
    if not value:
        return
    if isinstance(value, str):
        if value not in seen:
            seen.add(value)
            topics.append(value)
        return
    if isinstance(value, (list, tuple)):
        for item in value:
            if isinstance(item, str) and item and item not in seen:
                seen.add(item)
                topics.append(item)
        return


def collect_topics(cfg):
    topics = []
    seen = set()

    kafka_cfg = cfg.get("kafka", {}) if isinstance(cfg, dict) else {}
    if isinstance(kafka_cfg, dict):
        append_topic(topics, seen, kafka_cfg.get("topics"))
        append_topic(topics, seen, kafka_cfg.get("topic"))
        append_topic(topics, seen, kafka_cfg.get("retryTopic"))
        append_topic(topics, seen, kafka_cfg.get("deadLetterTopic"))

    status_cfg = cfg.get("status", {}) if isinstance(cfg, dict) else {}
    if isinstance(status_cfg, dict):
        append_topic(topics, seen, status_cfg.get("finalTopic"))

    submit_cfg = cfg.get("submit", {}) if isinstance(cfg, dict) else {}
    if isinstance(submit_cfg, dict):
        append_topic(topics, seen, submit_cfg.get("statusFinalTopic"))
        status_consumer = submit_cfg.get("statusFinalConsumer", {})
        if isinstance(status_consumer, dict):
            append_topic(topics, seen, status_consumer.get("deadLetterTopic"))

    cleanup_cfg = cfg.get("cleanup", {}) if isinstance(cfg, dict) else {}
    if isinstance(cleanup_cfg, dict):
        append_topic(topics, seen, cleanup_cfg.get("topic"))

    ban_event_cfg = cfg.get("banEvent", {}) if isinstance(cfg, dict) else {}
    if isinstance(ban_event_cfg, dict):
        append_topic(topics, seen, ban_event_cfg.get("topic"))

    return topics


def main() -> int:
    root = Path(__file__).resolve().parent.parent
    config_paths = [
        root / "configs" / "judge_service.yaml",
        root / "configs" / "submit_service.yaml",
        root / "configs" / "problem_service.yaml",
        root / "configs" / "gateway.yaml",
    ]

    topics = []
    seen = set()
    for path in config_paths:
        cfg = load_yaml(path)
        if not cfg:
            continue
        for topic in collect_topics(cfg):
            if topic not in seen:
                seen.add(topic)
                topics.append(topic)

    for topic in topics:
        print(topic)
    return 0


if __name__ == "__main__":
    sys.exit(main())
