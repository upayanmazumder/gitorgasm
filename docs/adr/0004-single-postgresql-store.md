# ADR 0004: Single PostgreSQL instance as the only stateful store

- Status: Accepted
- Date: 2026-10-26

## Context

The legacy app used Supabase Postgres and Upstash Redis. Only the dev Supabase project is alive. Redis was a cache only (**Verified** `docs/architecture/database-audit.md`). The cluster has no CloudNativePG, NATS, Valkey, Tempo, or ClickHouse (**Verified** `cluster-audit.md` §3). Memory is the binding constraint. There is one node, so replicas give no real high availability.

## Decision

- Run one PostgreSQL 18 instance (`postgres:18-alpine`) as a plain StatefulSet in namespace `gitorgasm` (**Verified** `deploy/kustomize/base/postgres.yaml`).
- Store data on a static hostPath PV at `/srv/data/gitorgasm-postgres` with reclaim policy `Retain`, pinned to node `vps`.
- Back up with the cluster restic mechanism: class A logical `pg_dump` every 30 minutes, hourly copy to Cloudflare R2 (`cluster-audit.md` §6).
- Keep sessions, locks, the job queue (River), webhook deliveries, and caches in Postgres.
- Budget: 100m / 256Mi requests, 1000m / 512Mi limits, `shared_buffers=64MB`, `max_connections=40`, `work_mem=4MB` (**Verified** manifest). Each core process opens at most `DB_MAX_CONNS=8` connections (ConfigMap `gitorgasm-core`).
- Not now: CNPG, Redis/Valkey, NATS, Temporal, ClickHouse. Search uses `pg_trgm` later. R2 is used for backups only.

## Consequences

- One stateful component to back up, restore, and upgrade.
- No point-in-time recovery. Offsite RPO is up to about 1.5 h (`docs/architecture/reliability.md` §4).
- Major version upgrades need dump and restore by hand (risk R16).
- Add a component only when a measurement shows the need, through a new ADR.

## Alternatives

- CloudNativePG: rejected now. Operator memory and complexity, and replicas on one node add no availability.
- Redis/Valkey for sessions, cache, or queue: rejected. Postgres covers these at current scale. Saves at least one pod.
- NATS for events: rejected. River covers work distribution. Webhook fan-out is not needed.
- Temporal for workflows: rejected. Server cost. River jobs with checkpoints suffice for Stage 4 bulk operations.
- ClickHouse for analytics: rejected. Legacy data is tiny (949 rows in the dev project). Use Postgres aggregates first.
- Reuse the legacy Supabase project: rejected. Only the dev project exists, and the budget forbids paid plans.
