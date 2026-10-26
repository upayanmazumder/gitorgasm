# ADR 0005: River Postgres-backed durable jobs with transactional enqueue

- Status: Accepted
- Date: 2026-10-26

## Context

Sync, webhook processing, and later bulk operations must survive crashes and restarts. GitHub rate limits require waiting until a reset time. Webhook data and the job that processes it must not diverge. ADR 0004 allows no extra broker.

## Decision

Use River (v0.49, `services/core/go.mod`) on the same PostgreSQL instance. **Verified** in `services/core/internal/jobs/jobs.go`:

- `sync_installation`: queue `github`, max 8 attempts, unique by args within a 1-minute period (`ByArgs` + `ByPeriod`), 15-minute timeout. One pass at a time per installation: the worker holds a session advisory lock with the 64-bit key `hashtextextended('gitorgasm:sync_installation:' || id, 0)` and snoozes 30 s when another pass holds it.
- `process_webhook`: max 10 attempts, unique by args (delivery id).
- `reconcile_installations`: periodic every `RECONCILE_INTERVAL` (default 6 h), `RunOnStart`.
- Rate limit → `river.JobSnooze` until the reset. 401, 403, and other client errors → `river.JobCancel`. Transient errors → River retry with backoff.
- Insert jobs in the same transaction as the related data change (for example the webhook row), so the database never holds data without its job or the reverse.
- The `migrate` role applies River's own migrations.

## Consequences

- No broker to run. Jobs are visible with SQL.
- Exactly-once processing is not guaranteed. Handlers must be idempotent (upserts, tombstone after a full pass).
- River has no Prometheus exporter. Job metrics need an event subscription (**Planned** Stage 2).
- Queue throughput is bounded by Postgres. Expected load is small (estimate: 10k repositories ≈ 102 requests per sync).

## Alternatives

- Hand-written `SELECT … FOR UPDATE SKIP LOCKED` queue: rejected. Retries, uniqueness, periodic jobs, and rescue of stuck jobs would need to be rebuilt.
- NATS JetStream or Redis streams: rejected by ADR 0004. No transactional enqueue with Postgres data.
- Temporal: rejected. Server cost and complexity.
