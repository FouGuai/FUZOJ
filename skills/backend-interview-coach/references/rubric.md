# Answer Rubric (评分与反馈)

Use this rubric to evaluate and improve the user's answers in `Mock` and `Review` mode.

## Scoring (0-2 each, total 10)

1) Correctness (0-2)
- 0: incorrect or internally inconsistent.
- 1: mostly correct but missing key constraints/edge cases.
- 2: correct with clear assumptions.

2) Structure & Clarity (0-2)
- 0: rambling; hard to follow.
- 1: some structure; key point not upfront.
- 2: TL;DR first; layered explanation; crisp terms.

3) Depth (0-2)
- 0: only definitions/buzzwords.
- 1: explains mechanism but shallow on tradeoffs.
- 2: explains mechanism + tradeoffs + when it breaks.

4) Practical/Production Thinking (0-2)
- 0: ignores operations.
- 1: mentions timeouts/retries/logging but not integrated.
- 2: covers failure modes, backpressure, observability, mitigation.

5) Follow-up Readiness (0-2)
- 0: cannot answer likely follow-ups.
- 1: can answer some follow-ups.
- 2: proactively addresses likely follow-ups.

## High-Value Feedback Pattern

- 1-2 sentences: what was good (specific).
- 3-5 bullets: missing dimensions (specific).
- 2 bullets: next practice targets (actionable drills).
- A model answer: short enough to memorize, but logically grounded.

## Red Flags to Call Out

- Mixing guarantees (e.g., "exactly once" without a dedup/transaction story).
- Overclaiming ordering (Kafka/global order, distributed order).
- Ignoring constraints (latency/QPS/data size) in design questions.
- Missing idempotency in any retried write path.
- Not distinguishing "per-partition ordering" vs "global ordering".

