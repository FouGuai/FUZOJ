# Answer Formats (Backend Interview)

Use these templates to keep answers structured, correct, and interview-friendly. Default language is Chinese; keep key terms in English in parentheses when helpful.

## 1) Knowledge / "What is X?"

1. One-sentence definition (what problem it solves).
2. Key properties (2-5 bullets).
3. How it works (the minimum internal mechanism needed to be correct).
4. Tradeoffs (when to use / when not to use).
5. Common pitfalls (what candidates often get wrong).
6. Follow-ups (3-6) + what they test.

## 2) Compare / "X vs Y"

1. Decision in one line: "If you need A, pick X; if you need B, pick Y."
2. Table-style bullets: semantics, performance, operability, ecosystem.
3. Edge cases: failure modes, scaling behavior, consistency implications.
4. Real-world examples: 1-2 scenarios per option.

## 3) "How would you troubleshoot..."

1. Restate symptoms + define success metric (SLO/SLA).
2. Triage: scope, recent changes, blast radius, rollback safety.
3. Hypothesis tree: client -> network -> service -> downstream -> DB/cache/MQ.
4. Observability: logs/metrics/traces; what signals confirm/refute hypotheses.
5. Mitigation first, then root cause: rate limit, shed load, circuit breaker, rollback.
6. Prevent recurrence: alerting, runbooks, capacity, tests.

## 4) Concurrency / "How do you ensure thread-safety / avoid races?"

1. Define correctness target: safety (no races) vs liveness (no deadlocks).
2. Choose primitive: mutex/RWLock/atomic/CAS/channel/queue.
3. Explain critical section and invariant (what must remain true).
4. Mention hazards: deadlock, starvation, priority inversion, false sharing.
5. Testing: race detector, stress tests, deterministic tests when possible.

## 5) Database / "Index / transaction / isolation / query optimization"

1. Model the access pattern (read/write ratio, query shapes, latency budget).
2. Explain underlying mechanism briefly:
   - Index: B+Tree, selectivity, covering index, leftmost prefix.
   - Transaction: ACID, MVCC, locks, isolation anomalies.
3. Show tradeoffs: write amplification, storage, lock contention, hotspots.
4. Practical guidance: EXPLAIN/ANALYZE, slow logs, query rewrite, schema changes.
5. Pitfalls: N+1, missing predicates, non-sargable expressions, huge offsets.

## 6) Cache / Redis

1. Clarify goal: latency, DB offload, hot data, rate limiting, distributed lock.
2. Choose pattern:
   - Cache-aside (read-through by app), write-through, write-behind.
   - TTL strategy, null caching (avoid penetration).
3. Failure modes: avalanche, breakdown, penetration, stale reads, hot keys.
4. Consistency: invalidation vs update, eventual consistency expectations.
5. Ops: eviction policy, memory sizing, replication, persistence tradeoffs.

## 7) Messaging / Kafka (or MQ in general)

1. Semantics: at-most-once / at-least-once / exactly-once (EOS).
2. Ordering: per-partition ordering vs global ordering.
3. Delivery mechanics: ack, retries, idempotency, dedup strategy.
4. Backpressure: consumer lag, batch size, timeouts, rebalancing.
5. Failure handling: DLQ, poison message, replay, schema evolution.

## 8) API Design

1. Define resource model (nouns), operations (verbs), idempotency.
2. Error model: stable error codes, retriable vs non-retriable.
3. Pagination, filtering, sorting; consistency guarantees.
4. Versioning strategy; backward compatibility.
5. Security: authn/authz, rate limiting, audit logs.

## 9) Behavioral / Project Deep Dive

### Behavioral (STAR)
1. Situation/Task: context + stakes.
2. Action: what you did (decisions + tradeoffs), not just "we".
3. Result: numbers and impact; what you'd improve next time.
4. Follow-ups: conflicts, failures, feedback, ownership.

### Project Deep Dive
1. Problem and constraints (latency/QPS/data size/cost).
2. Architecture: key components and why.
3. Two hard problems you solved (tradeoffs + alternatives).
4. Reliability/observability: timeouts, retries, dashboards, oncall learnings.
5. Postmortem: one incident and what changed afterward.

