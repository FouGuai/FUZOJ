# Backend Fundamentals (Quick Reminders)

This is a lightweight reminder list, not a specification. If details are version-dependent, state assumptions.

## Distributed Systems

- CAP tradeoff: consistency vs availability under partition; most systems pick per-operation semantics.
- Consistency patterns: quorum, leader/follower, read-your-writes, monotonic reads.
- Idempotency: essential with retries; use idempotency keys and dedup storage.
- Backpressure: protect downstream; bounded queues, rate limiting, circuit breakers.

## Caching

- Cache-aside: app reads cache, on miss reads DB then sets cache; invalidation on writes.
- Cache penetration/breakdown/avalanche: mitigate with null cache, singleflight/coalescing, jittered TTL, local hot cache.
- Hot keys: shard, local cache, request coalescing, protect origin.

## Databases

- Indexes: design for access patterns; selectivity matters; avoid over-indexing on write-heavy tables.
- Transactions: understand anomalies; isolation impacts throughput and correctness.
- Query optimization: measure with EXPLAIN, slow logs; rewrite queries; add/adjust indexes.

## Messaging (Kafka/MQ)

- Semantics: at-least-once is common; duplicates must be handled downstream.
- Ordering: typically per-partition; key choice affects ordering and load balance.
- Consumer lag: indicates insufficient consumption capacity or downstream slowness.
- DLQ and replay: plan for poison messages and reprocessing.

## Networking / OS

- Timeouts everywhere (client, server, DB, MQ); avoid unbounded waits.
- Connection pools: size for throughput; watch for head-of-line blocking.
- Threading model: avoid blocking in event loops; bound goroutines/threads.

## System Design Interview Meta

- Answer-first: start with the simplest correct high-level design, then deepen.
- Make assumptions explicit; quantify; list tradeoffs; address failure modes.

