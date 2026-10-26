# Rust vs Go

This document selects a language per logical service. It uses measured benchmark numbers and the workload model. It defines the rules for a polyglot system.

## 1. Summary

| Service                     | Language                | State                                                                                |
| --------------------------- | ----------------------- | ------------------------------------------------------------------------------------ |
| core api                    | Go                      | **Verified** implemented: `core` (`api` role)                                        |
| worker / sync               | Go                      | **Verified** implemented: `core` (`worker` role)                                     |
| webhook ingest              | Go (inside core api)    | **Verified** implemented: `POST /webhooks/github` in `core/internal/httpapi/orgs.go` |
| operations (bulk mutations) | Go (inside core worker) | Planned, Stage 4                                                                     |
| scoring engine              | Rust                    | **Hypothesis**. Planned, Stage 5. Re-verify before building (section 5).             |
| analytics                   | Go + SQL rollups        | Planned, Stage 5+                                                                    |

## 2. Measured benchmark

Source: [`tests/performance/lang-bench/RESULTS.md`](../../tests/performance/lang-bench/RESULTS.md). All values below are **Verified** there.

### 2.1 Environment

- AMD Ryzen 7 7840U, 16 logical CPUs, 14.1 GB RAM, CachyOS Linux 7.3, AC power, governor `performance`.
- Go 1.27.1 (stdlib only). Rust 1.97.1 (serde_json 1.0.154, axum 0.8.9, tokio 1.53.2), release, no LTO.
- Background load average 8.5–12.3 during the runs. Two runs (A, B).
- Fixture 9.78 MB synthetic repo JSON. Both languages produce identical output checksums.

### 2.2 Results

| Workload                             | Go                                                              | Rust                  | Rust / Go     |
| ------------------------------------ | --------------------------------------------------------------- | --------------------- | ------------- |
| Parse 10k repos (median, A / B)      | 42.5–60.4 ms (`encoding/json` v1 and v2, `GOMAXPROCS` 1 and 16) | 20.1 / 24.6 ms        | ~0.4–0.5×     |
| Score 1M events + top 100 (A / B)    | 27.2–35.7 ms                                                    | 13.6 / 14.8 ms        | ~0.4–0.5×     |
| Score allocations per op             | 47,704                                                          | 16                    | —             |
| Hello service RSS at 1 s             | 11.4 MB                                                         | 3.7 MB                | ~0.32×        |
| Hello service RSS after 10k requests | 16.1–16.8 MB                                                    | 3.9–4.0 MB            | ~0.24×        |
| Stripped binary, hello service       | 5.9 MB                                                          | 1.2 MB                | ~0.2×         |
| 10k sequential requests p50/p99      | Runs A and B disagree                                           | Runs A and B disagree | No conclusion |

### 2.3 Limitations (from RESULTS.md)

- Synthetic data. 27 fields per repo instead of about 80 in real responses.
- One laptop CPU, not the target Hetzner vCPU. Absolute numbers will differ.
- Background load. Only differences of about 2× or more are reliable.
- No container limits, TLS, database, or concurrent clients.
- Go map stores pointers in the score workload. A value map would cut allocations.

### 2.4 What the numbers mean here

- Parse cost per 100-repo GitHub page: ~0.5 ms Go, ~0.2 ms Rust. Network time per page: ~100–500 ms (**Assumption**). The language changes sync time by < 1%. **Decision**: CPU speed does not decide the sync, api, or webhook language.
- Scoring 1M events: 15–35 ms in both. **Assumption**: real signals with rule evaluation, quotas, diminishing returns, and 6 periods cost more per event. The 2× gap matters only if scoring runs often over large orgs.
- Memory: Rust saves ~8–12 MiB per idle process. With 2–4 pods this is 20–50 MiB of a ~1.5Gi headroom. Small advantage.

## 3. Decision matrix

Scale: ++ strong advantage, + advantage, 0 neutral, − disadvantage. Each cell compares Go to Rust **for that service**.

### 3.1 Per-service dominant workload

| Service        | Dominant workload              | CPU              | I/O                       | Memory                | Tail latency driver          |
| -------------- | ------------------------------ | ---------------- | ------------------------- | --------------------- | ---------------------------- |
| core api       | DB reads, session lookup, JSON | Low              | Postgres                  | Small                 | Postgres query time and pool |
| worker / sync  | GitHub REST pages, upserts     | Low              | GitHub network, Postgres  | Small                 | GitHub latency, rate limits  |
| webhook ingest | HMAC + 1 insert transaction    | Low              | Postgres                  | Bounded by 25 MiB cap | Postgres commit              |
| operations     | Paced GitHub mutations         | Very low         | GitHub, paced at ≤ 80/min | Small                 | Secondary limits             |
| scoring engine | Aggregation over signals       | **High per run** | Postgres bulk read/write  | Grows with signals    | Batch duration               |
| analytics      | SQL rollups                    | In Postgres      | Postgres                  | In Postgres           | Query plans                  |

### 3.2 Criteria (Go column relative to Rust)

| Criterion               | core api                                                          | worker/sync | webhook | operations | scoring                                         | analytics    |
| ----------------------- | ----------------------------------------------------------------- | ----------- | ------- | ---------- | ----------------------------------------------- | ------------ |
| CPU efficiency          | 0                                                                 | 0           | 0       | 0          | − (Rust ~2×)                                    | 0            |
| I/O handling            | 0                                                                 | 0           | 0       | 0          | 0                                               | 0            |
| Memory footprint        | −                                                                 | −           | −       | −          | −                                               | −            |
| Tail latency (GC)       | 0 (DB dominates)                                                  | 0           | 0       | 0          | − (47k allocs/op vs 16)                         | 0            |
| Runtime overhead        | − (~11 MB vs ~4 MB idle)                                          | −           | −       | −          | −                                               | −            |
| Concurrency model       | + goroutines, simple                                              | +           | +       | +          | 0                                               | 0            |
| GitHub client maturity  | + `go-github` mature; Gitorgasm uses a narrow hand-written client | +           | 0       | +          | n/a                                             | n/a          |
| Queue library           | ++ River is Go-only                                               | ++          | ++      | ++         | − (Rust must consume rows without River client) | +            |
| DB library              | + pgx (used); sqlc optional                                       | +           | +       | +          | 0 (SQLx mature)                                 | +            |
| Dev productivity        | +                                                                 | +           | +       | +          | 0                                               | +            |
| Build time              | +                                                                 | +           | +       | +          | 0                                               | +            |
| Testing                 | + stdlib `testing`, real Postgres per test (`internal/testdb`)    | +           | +       | +          | 0 (golden tests in either)                      | +            |
| Debugging               | + delve, pprof                                                    | +           | +       | +          | 0                                               | +            |
| Container footprint     | − (5.9 MB vs 1.2 MB stripped)                                     | −           | −       | −          | −                                               | −            |
| Contributor familiarity | + (legacy team writes TypeScript; Go is closer)                   | +           | +       | +          | −                                               | +            |
| Maintenance             | + one toolchain                                                   | +           | +       | +          | − second toolchain                              | +            |
| Ops complexity          | + one image, three roles                                          | +           | +       | +          | − second image, second CI                       | +            |
| **Verdict**             | **Go**                                                            | **Go**      | **Go**  | **Go**     | **Rust (hypothesis)**                           | **Go + SQL** |

Library facts:

- River: Go only. **Verified**: `github.com/riverqueue/river` in `core/go.mod`. No Rust client exists in that project (**Assumption** from the repository contents).
- GitHub clients: `go-github` (Go) and `octocrab` (Rust) both exist. **Decision**: Gitorgasm uses a narrow hand-written client for the endpoints it needs (D12, `core/internal/github/client.go`), so library maturity matters less.
- Database: Gitorgasm uses `pgx/v5` with hand-written SQL in one package (`core/internal/store/store.go`). `sqlc` is an option, not used. Rust would use SQLx.

## 4. Polyglot rules

These rules apply when the first non-Go service exists. They apply now to any new process.

| Topic           | Rule                                                                                                                                                                                                                                                                          |
| --------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Boundaries      | One service owns a domain's tables for writing. Other services read through views or documented tables only. Scoring writes `computed_scores` and reads `signals`.                                                                                                            |
| Contract format | Postgres tables and job rows are the contract between core and scoring. Schema changes go through the goose migrations in `core/internal/db/migrations`. If a synchronous RPC is ever needed: ConnectRPC with protobuf (D6). Public API: OpenAPI `contracts/openapi/v1.yaml`. |
| Serialization   | JSON in `jsonb` and job args. Field names `snake_case`. Unknown fields are ignored by readers. Fields are added, never renamed or retyped in place. Integers that may exceed 2^53 travel as strings in JSON.                                                                  |
| Compatibility   | Expand then contract: add the column or field, deploy readers, deploy writers, remove old fields one release later. Every job row carries a `version` in args when its shape changes.                                                                                         |
| Job handoff     | Core enqueues work for scoring as rows the Rust service can claim with `SELECT … FOR UPDATE SKIP LOCKED`, or as a simple jobs table. Rust does not link the River client.                                                                                                     |
| Logging         | JSON to stdout. Keys: `time`, `level`, `msg`, `service`, `component`, `app_version`, `request_id`, `trace_id`, `span_id`. Same as `core/internal/obs/obs.go` and `cmd/gitorgasm/main.go`.                                                                                     |
| Tracing         | W3C Trace Context. OTLP/HTTP export only when `OTEL_EXPORTER_OTLP_ENDPOINT` is set. Job rows carry `traceparent` in metadata so a job span links to its request span (Planned).                                                                                               |
| Metrics         | Prometheus on `:9090`, prefix `gitorgasm_`, labels with bounded cardinality.                                                                                                                                                                                                  |
| Health          | `/healthz`: process alive, no dependency check. `/readyz`: Postgres reachable. Same semantics as `obs.MetricsServer` and `httpapi.Server`.                                                                                                                                    |
| Auth context    | Background services do not receive user sessions. Job rows carry `actor_user_id` and `installation_id`. The enqueuing service already checked authorization. The consumer re-checks tenant scope in SQL.                                                                      |
| Images          | One image per service, distroless, non-root, read-only root filesystem. Independent versions and release tags.                                                                                                                                                                |
| FFI             | None. No cgo, no Rust linked into Go, no Go linked into Rust.                                                                                                                                                                                                                 |
| CI              | Each language has its own job. Rust CI is added only when the Rust service exists (D16).                                                                                                                                                                                      |

## 5. Rust scoring: hypothesis and benchmark plan

**Hypothesis**: the scoring engine is CPU-bound over large signal sets, and Rust's ~2× speed, lower allocations, and lower RSS reduce batch time and memory spikes on the shared node.

Facts against an early decision:

- Legacy dev data has 50 `signals` rows (database-audit.md). The 1M-event benchmark is synthetic.
- Scoring runs in a background job, not per request (**Assumption**). 15–35 ms per 1M events may be irrelevant.
- A second language adds a toolchain, image, CI job, and contract surface.

**Planned** benchmark before Stage 5 build:

1. Port legacy `lib/scoring` to Go and Rust behind the same golden tests (legacy-audit §9).
2. Generate signal sets with legacy distributions (types, per-user counts, 6 periods, quotas, diminishing returns) at 10k, 1M, and 10M events.
3. Measure end to end: read signals from Postgres, score, write `computed_scores` in one transaction.
4. Run on the target node `vps` with the planned container limits (CPU 500m, memory 256Mi).
5. Record p50/p95 batch time, peak RSS, and allocations. Use at least 5 runs and report variance.

**Decision rule**: build in Rust only if the Go version misses the batch target or exceeds the memory limit on the largest realistic org, and Rust meets both. Otherwise keep scoring in Go inside the worker.
