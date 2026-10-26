# ADR 0003: Go for the core, Rust reserved for the scoring engine

- Status: Accepted
- Date: 2026-10-26

## Context

Two languages were candidates for the backend. Measurements exist in `tests/performance/lang-bench/RESULTS.md` (**Verified**, two runs on 2026-10-26, synthetic data, developer laptop, not the cluster node):

| Workload                          | Go                     | Rust                     |
| --------------------------------- | ---------------------- | ------------------------ |
| Parse 10k repos                   | about 42–60 ms         | about 20–25 ms           |
| Score 1M events + top 100         | 27.2–35.7 ms (medians) | 13.6 / 14.8 ms (medians) |
| Score allocations per op          | 47,704                 | 16                       |
| Hello service RSS 1 s after start | about 11.4 MB          | about 3.7 MB             |

Per page of 100 repositories, parse cost is about 0.5 ms (Go) or 0.2 ms (Rust) (**Assumption** derived in `RESULTS.md`). GitHub calls take tens to hundreds of milliseconds. Sync is network- and rate-limit-bound.

## Decision

- Use Go for `core` (API, jobs, GitHub client). Reasons: the workload is I/O-bound. River (Postgres-backed durable queue with transactional enqueue, ADR 0005) exists only for Go. The standard library HTTP stack is strong. Builds are fast. Contributors from the TypeScript legacy can learn it quickly.
- Use Rust for `scoring`: the leaderboard scoring engine. Scoring is CPU-bound aggregation, where Rust took about 40–50% of the Go time and allocated far less.
- The future scoring process communicates only through PostgreSQL (reads signals, writes `computed_scores`, consumes job rows). No FFI. No synchronous Go-to-Rust calls.
- **Done (2026-10-26):** `scoring/` is a pure library crate. It ports legacy `lib/scoring` (engine and aggregation). Golden tests in `scoring/tests/golden` compare every output bit for bit with the legacy TypeScript engine, on 6 synthetic fixtures. A one-off local run on the 50 legacy dev signals also matched. The crate has no I/O, HTTP, or database code.
- **Planned (Stage 5):** a binary that reads `signals`, runs the crate, and writes `computed_scores`. Build it when signal ingest exists. Before that, an empty service would be scaffolding without work.

## Consequences

- Two backend languages from Stage 1. CI runs a separate `scoring` job (fmt, clippy pedantic, tests, cargo-deny).
- The Rust decision is still a hypothesis for production cost. Re-run the benchmark on the crate with legacy-shaped data on node `vps` before Stage 5 (risk R15). If the gain is not material, keep the golden fixtures, port the crate to Go, and supersede this ADR.

## Alternatives

- Rust for everything: rejected. No mature equivalent of River with transactional enqueue on Postgres. The performance gain does not matter for I/O-bound work.
- TypeScript (Node) like the legacy: rejected. Higher memory per process on a memory-bound node, and the legacy defects came partly from the unstructured server code (`legacy-audit.md`).
- Go for scoring too: kept as the fallback described above.
