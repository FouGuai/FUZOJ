# System Design Checklist

Use this as a flow for system design interviews. Do not skip assumptions.

## 0) Clarify and Lock Requirements

- Functional requirements (core user stories).
- Non-functional requirements:
  - latency (p95/p99), throughput (QPS), availability (SLA), durability,
  - consistency requirements (strong vs eventual; read-your-writes),
  - geo (single-region vs multi-region), cost constraints.
- Out of scope explicitly.

## 1) Capacity Estimation (lightweight but concrete)

- QPS, read/write ratio, peak factor.
- Data size growth: items/day, bytes/item, retention.
- Bandwidth: ingress/egress.
- Hot key/hot partition risk.

## 2) APIs and Data Model

- API shapes: endpoints or RPCs; request/response; idempotency key if needed.
- Data model: entities, primary keys, indexes, access patterns.
- Consistency boundary: what must be atomic; what can be eventual.

## 3) High-Level Architecture

- Components: gateway, stateless services, storage, cache, queue/stream, search, blob store.
- Data flow for main operations (write path, read path).
- Choose storage based on access patterns:
  - relational (transactions), KV (simple lookups), document (flexible schema), columnar (analytics).

## 4) Scaling Strategy

- Stateless horizontal scaling; autoscaling signals.
- Partitioning/sharding strategy:
  - by user_id/tenant_id/time; balance vs locality; re-sharding plan.
- Caching:
  - cache-aside, TTL, invalidation, hot key mitigation (local cache, request coalescing).
- Async:
  - queues for non-critical path; batch processing; backpressure.

## 5) Reliability and Failure Modes

- Timeouts (end-to-end budget), retries (with jitter), circuit breaker.
- Idempotency for retried writes; dedup for at-least-once delivery.
- Degrade gracefully: feature flags, partial responses, load shedding.
- Data safety:
  - backups, restore drills, schema migrations, replay strategy.
- Typical failure modes to address:
  - downstream outage, partial network partition, thundering herd, hot partition,
  - message backlog, cache outage, clock skew (for time-based logic).

## 6) Observability and Operations

- SLIs/SLOs: latency, error rate, saturation, queue lag, cache hit ratio.
- Logs/metrics/traces; correlation IDs; structured logs.
- Alerting: symptom-based first; runbooks; dashboards for oncall.

## 7) Security and Privacy (minimum viable)

- Authentication/authorization; least privilege; audit logs.
- Rate limiting; abuse prevention; input validation.
- Data encryption in transit/at rest; secret management.

## 8) Tradeoffs Summary

- 3-6 explicit tradeoffs and why you chose them.
- What you'd do differently at 10x scale / multi-region / stricter consistency.

