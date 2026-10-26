# ADR 0003: Go for the core, Rust reserved for the scoring engine

- Status: Accepted
- Date: 2026-10-26

## Context

Two languages were candidates for the backend. Measurements exist in `tests/performance/lang-bench/RESULTS.md` (**Verified**, two runs on 2026-10-26, synthetic data, developer laptop, not the cluster node):

| Workload | Go | Rust |
| --- | --- | --- |
| Parse 10k repos | about 42–60 ms | about 20–25 ms |
| Score 1M events + top 100 | 27.2–35.7 ms (medians) | 13.6 / 14.8 ms (medians) |
| Score allocations per op | 47,704 | 16 |
| Hello service RSS 1 s after start | about 11.4 MB | about 3.7 MB |

Per page of 100 repositories, parse cost is about 0.5 ms (Go) or 0.2 ms (Rust) (**Assumption** derived in `RESULTS.md`). GitHub calls take tens to hundreds of milliseconds. Sync is network- and rate-limit-bound.

## Decision

- Use Go for `core` (API, jobs, GitHub client). Reasons: the workload is I/O-bound. River (Postgres-backed durable queue with transactional enqueue, ADR 0005) exists only for Go. The standard library HTTP stack is strong. Builds are fast. Contributors from the TypeScript legacy can learn it quickly.
- Reserve Rust for `scoring` (**Planned**, Stage 5): the leaderboard scoring engine. Scoring is CPU-bound aggregation, where Rust took about 40–50% of the Go time and allocated far less.
- The scoring service communicates only through PostgreSQL (reads signals, writes `computed_scores`, consumes job rows). No FFI. No synchronous Go-to-Rust calls.
- Do not create `scoring` before Stage 5. An empty service would be fake scaffolding.

## Consequences

- One language until Stage 5. Simpler CI and onboarding.
- The Rust decision is a hypothesis. Re-run the benchmark with legacy-shaped data before Stage 5 (risk R15). If the gain is not material at real data sizes, implement scoring in Go and supersede this ADR.
- Rust CI is added only when the Rust service exists.

## Alternatives

- Rust for everything: rejected. No mature equivalent of River with transactional enqueue on Postgres. The performance gain does not matter for I/O-bound work.
- TypeScript (Node) like the legacy: rejected. Higher memory per process on a memory-bound node, and the legacy defects came partly from the unstructured server code (`legacy-audit.md`).
- Go for scoring too: kept as the fallback described above.
