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


def pick_cfg(cfg, *keys):
    if not isinstance(cfg, dict):
        return {}
    for key in keys:
        if key in cfg and isinstance(cfg[key], dict):
            return cfg[key]
    return {}


def collect_topics(cfg):
    topics = []
    seen = set()

    direct_topics = {}
    if isinstance(cfg, dict):
        direct_topics = cfg.get("topics", cfg.get("Topics", {}))
    if isinstance(direct_topics, dict):
        for value in direct_topics.values():
            append_topic(topics, seen, value)
    else:
        append_topic(topics, seen, direct_topics)

    kafka_cfg = pick_cfg(cfg, "kafka", "Kafka")
    if kafka_cfg:
        append_topic(topics, seen, kafka_cfg.get("topics"))
        append_topic(topics, seen, kafka_cfg.get("Topics"))
        append_topic(topics, seen, kafka_cfg.get("topic"))
        append_topic(topics, seen, kafka_cfg.get("Topic"))
        append_topic(topics, seen, kafka_cfg.get("retryTopic"))
        append_topic(topics, seen, kafka_cfg.get("RetryTopic"))
        append_topic(topics, seen, kafka_cfg.get("deadLetterTopic"))
        append_topic(topics, seen, kafka_cfg.get("DeadLetterTopic"))

    status_cfg = pick_cfg(cfg, "status", "Status")
    if status_cfg:
        append_topic(topics, seen, status_cfg.get("finalTopic"))
        append_topic(topics, seen, status_cfg.get("FinalTopic"))

    submit_cfg = pick_cfg(cfg, "submit", "Submit")
    if submit_cfg:
        append_topic(topics, seen, submit_cfg.get("statusFinalTopic"))
        append_topic(topics, seen, submit_cfg.get("StatusFinalTopic"))
        status_consumer = submit_cfg.get("statusFinalConsumer", {})
        if not status_consumer:
            status_consumer = submit_cfg.get("StatusFinalConsumer", {})
        if isinstance(status_consumer, dict):
            append_topic(topics, seen, status_consumer.get("deadLetterTopic"))
            append_topic(topics, seen, status_consumer.get("DeadLetterTopic"))

    cleanup_cfg = pick_cfg(cfg, "cleanup", "Cleanup")
    if cleanup_cfg:
        append_topic(topics, seen, cleanup_cfg.get("topic"))
        append_topic(topics, seen, cleanup_cfg.get("Topic"))

    ban_event_cfg = pick_cfg(cfg, "banEvent", "BanEvent")
    if ban_event_cfg:
        append_topic(topics, seen, ban_event_cfg.get("topic"))
        append_topic(topics, seen, ban_event_cfg.get("Topic"))

    return topics


def main() -> int:
    root = Path(__file__).resolve().parent.parent
    configs_root = root / "configs"
    config_paths = []
    if configs_root.exists():
        for path in sorted(configs_root.rglob("*.yaml")):
            if "dev.generated" in path.parts:
                continue
            config_paths.append(path)

    services_root = root / "services"
    if services_root.exists():
        for path in sorted(services_root.rglob("etc/*.yaml")):
            if "dev.generated" in path.parts:
                continue
            config_paths.append(path)

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
