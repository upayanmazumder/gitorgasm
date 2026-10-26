# Data architecture

Status: Stage 1 (Foundation). This document describes the data in `core` and the planned data for later stages.

Labels:

- **Verified**: a file path, a command, or an official URL proves the statement.
- **Assumption**: not proven. Check it before you rely on it.
- **Decision**: a choice for Gitorgasm. Source: integrator decisions (`decisions.md`, items 4, 5, 13, 18).
- **Alternative considered**: an option that we did not choose, with the reason.
- **Planned**: not implemented. A later stage adds it.

## 1. Storage topology

**Decision**: one PostgreSQL 18 instance is the only stateful service. It holds application tables, GitHub mirror tables, sessions, the webhook inbox, the audit trail, and the River job queue. Restic copies logical dumps to Cloudflare R2.

```mermaid
flowchart LR
  subgraph k3s["k3s node vps, namespace gitorgasm"]
    API["core role=api"]
    WRK["core role=worker"]
    MIG["core role=migrate (Job)"]
    subgraph PG["PostgreSQL 18 StatefulSet"]
      APP[("public: app + mirror tables")]
      RQ[("public: river_job, river_queue,<br/>river_leader, river_migration,<br/>river_client, river_client_queue")]
    end
  end
  GH["GitHub REST API"]
  R2[("Cloudflare R2<br/>restic pg_dump")]
  API -- "SQL, InsertTx" --> APP
  API -- "InsertTx" --> RQ
  WRK -- "fetch / complete jobs" --> RQ
  WRK -- "upsert mirror" --> APP
  WRK -- "installation + app tokens" --> GH
  API -- "user token at login" --> GH
  MIG -- "goose + rivermigrate" --> PG
  PG -. "pg_dump every 30 min, upload hourly" .-> R2
```

- **Verified**: `core/internal/db/db.go` runs goose migrations first, then `rivermigrate` (`Migrate`). Both take a PostgreSQL advisory lock, so concurrent runs are safe (`lock.NewPostgresSessionLocker`).
- **Verified**: the River table set comes from River's own migrations, not from `00001_core.sql`. The exact River table list depends on the River version in `core/go.mod`. **Assumption**: the names in the diagram match River v0.x. Check with `\dt river_*` after `make migrate`.
- **Decision**: no Redis, NATS, Temporal, or ClickHouse. Reasons are in `decisions.md` item 4.
- **Alternative considered**: CloudNativePG. Rejected: operator overhead, and the single node gives no real HA.

## 2. Table ownership and classification

Categories:

- **Authoritative**: Gitorgasm is the only source. Loss means data loss.
- **Mirror**: a copy of GitHub data. GitHub stays the source. A full sync rebuilds it.
- **Derived**: computed from other tables. A recompute rebuilds it.
- **Cache / ephemeral**: safe to drop. Users log in again or jobs re-run.

Source: `core/internal/db/migrations/00001_core.sql`. **Verified**.

| Table | Class | Writer (code) | Reader (code) | Notes |
|---|---|---|---|---|
| `users` | Mirror (GitHub identity) + internal id | `store.UpsertUser` at login (`httpapi/auth.go` `callback`) | `store.LookupSession` | `id` is internal. `github_id` is unique. |
| `sessions` | Authoritative, ephemeral | `store.CreateSession`, `store.DeleteSession`, `store.DeleteSessionsForGitHubUser` | `store.LookupSession` | Stores `SHA-256(session id)` and sealed GitHub tokens. Loss = users log in again. |
| `accounts` | Mirror | `store.UpsertInstallation` | `store.ListOrganizations`, `store.AccessibleInstallation` | Unique index on `lower(login)`. |
| `installations` | Mirror | `store.UpsertInstallation`, `store.MarkInstallationDeleted` | list and access queries | Soft delete with `deleted_at`. |
| `user_installations` | Authoritative grant, sourced from GitHub | `store.ReplaceUserInstallations` (login), `store.MarkInstallationDeleted` | every org-scoped query | The tenant boundary. See section 6. |
| `repositories` | Mirror | `store.UpsertRepositories`, `store.TombstoneUnseenRepositories` | `store.ListRepositories`, repo count in `ListOrganizations` | Tombstoned with `deleted_at`. |
| `sync_state` | Derived (sync metadata) | `store.SyncStarted`, `store.SyncFinished` | `ListOrganizations` (`last_succeeded_at`) | Safe to drop. Next sync recreates it. |
| `webhook_deliveries` | Authoritative inbox (short-lived) | `store.InsertWebhookDelivery`, `store.FinishWebhookDelivery` | `store.GetWebhookDelivery` | Raw signed body as `BYTEA`. Dedupe key = `X-GitHub-Delivery`. |
| `audit_events` | Authoritative, append-only | `store.Audit` | none yet | Actions in use: `session.created`, `installation.sync_requested`, `session.revoked_by_github`. |
| `river_*` | Authoritative work state | River client (`jobs` package, `httpapi`) | River worker | Job kinds: `sync_installation`, `process_webhook`, `reconcile_installations` (`internal/jobs/jobs.go`). |

**Verified**: the migration comment says "GitHub-owned data (accounts, installations, repositories) is a mirror. GitHub stays the source of truth."

**Decision**: the backup class for the whole database is class A (decisions item 4). Mirror tables are in the backup only because one logical dump is simpler than a selective dump.

## 3. Transaction boundaries

Every boundary below is in code. **Verified** by the cited function.

| Use case | One transaction contains | Code |
|---|---|---|
| Login callback | upsert user, upsert each installation and account, replace grants, create session, `InsertManyTx` of `sync_installation` jobs, audit `session.created` | `httpapi/auth.go` `callback` (`pgx.BeginFunc`) |
| Manual sync request | `InsertTx` of `sync_installation`, audit `installation.sync_requested` | `httpapi/orgs.go` `requestSync` |
| Webhook receive | insert `webhook_deliveries` row, `InsertTx` of `process_webhook` | `httpapi/orgs.go` `webhook` |
| Webhook apply | event-specific writes, `InsertTx` of follow-up `sync_installation` | `jobs/jobs.go` `WebhookWorker.apply` |
| Repository page | one multi-row `INSERT … ON CONFLICT` per page of 100 | `store.UpsertRepositories` |

Not in one transaction (**Verified**, `jobs/jobs.go` `SyncWorker.Work`):

- A full sync is many short statements, one per page. A crash between pages leaves some rows with the new `synced_at`. This is safe: tombstoning runs only after every page succeeded (section 7).
- `ReconcileWorker` calls `client.InsertMany` outside a transaction. This is safe: it only reads `installations` and inserts unique jobs.

**Decision**: no GitHub call runs inside an open database transaction. The login callback calls GitHub first and opens the transaction after. This keeps transactions short and avoids holding locks during network I/O.

## 4. Transactional outbox with River

**Verified** (`jobs/jobs.go` package comment): "River stores jobs in PostgreSQL, so a job is committed atomically with the data change that caused it (transactional outbox)."

- A job row exists if and only if its cause committed. No separate outbox table and no relay process exist.
- Dedupe: `SyncInstallationArgs` and `ProcessWebhookArgs` use `UniqueOpts{ByArgs: true}`. A second insert while a matching job is pending returns `UniqueSkippedAsDuplicate`. `requestSync` returns this as `duplicate: true`.
- Retry policy: `sync_installation` has `MaxAttempts: 8` and a 15-minute timeout. `process_webhook` has `MaxAttempts: 10`. Rate limits snooze the job. 401, 403, and other 4xx cancel it (`retryDecision`).
- **Alternative considered**: NATS JetStream as an event bus. Rejected: a second stateful system for no current consumer (decisions item 4).

## 5. Retention

No purge job exists in the slice. **Verified**: no periodic job other than `reconcile_installations` is registered in `jobs.NewWorkerClient`.

| Data | Growth driver | Retention | Mechanism |
|---|---|---|---|
| `sessions` | one row per login | delete rows with `expires_at < now()` | **Planned**: periodic River job `purge_sessions` (daily). Index `sessions_expires_at_idx` exists. |
| `webhook_deliveries` | one row per delivery | keep processed rows 14 days. Keep unprocessed rows until they succeed or 30 days. | **Planned**: periodic job `purge_webhook_deliveries` (daily). Index `webhook_deliveries_unprocessed_idx` covers unprocessed rows only. A `received_at` index is needed for the purge. Add it in the same migration. |
| `audit_events` | one row per security action | keep 400 days | **Planned**: periodic job `purge_audit_events` (weekly). **Alternative considered**: monthly partitions with `DROP PARTITION`. Defer until the table exceeds about 10 M rows. |
| `river_job` (completed, cancelled, discarded) | every job | River job cleaner retention | **Assumption**: River's built-in job cleaner deletes finalized jobs with its default retention. `river.Config` in `jobs.NewWorkerClient` sets no retention values. Verify the defaults for the pinned River version before production. |
| `repositories` tombstones | deleted repos | keep 30 days, then hard delete | **Planned**: part of `purge_*` maintenance. |

**Decision**: each purge job deletes in batches of at most 10,000 rows per statement and logs the row count. This avoids long locks on the single node.

## 6. Tenant isolation in queries

**Decision**: tenant = GitHub installation. The grant edge is `user_installations` (decisions item 9).

**Verified** (`core/internal/store/store.go`):

- `ListOrganizations` starts `FROM user_installations ui … WHERE ui.user_id = $1`. It also requires `i.deleted_at IS NULL`.
- `AccessibleInstallation` joins `user_installations ui ON … AND ui.user_id = $1`. A missing grant returns `ErrNotFound`. The handler maps it to HTTP 404 `organization_not_found` (`httpapi/orgs.go` `resolveOrg`).
- `ListRepositories` filters by `installation_id = $1`. The handler passes only an id that `AccessibleInstallation` returned.
- Every statement uses bind parameters (`$1`, `$2`, …). No handler builds SQL (package comment of `store`).
- `ReplaceUserInstallations` deletes grants that GitHub no longer reports, in the login transaction.
- `MarkInstallationDeleted` deletes all grants for a removed installation.

Rules for new code (**Decision**):

1. Every org-scoped query takes an installation id that came from `AccessibleInstallation` or joins `user_installations` itself.
2. Workers act on an installation id from a job argument. Workers never act on a user request directly.
3. Code review rejects a new store function that reads tenant data without one of the two patterns above.

**Planned** (Stage 6): PostgreSQL row-level security as a second layer. **Alternative considered**: RLS now. Deferred: one service role, one code path, and the store package already centralizes SQL. RLS adds per-connection `SET` overhead and complexity without a second client.

## 7. Reconciliation

GitHub is the source for mirror tables. Gitorgasm converges on GitHub state through three paths. **Verified** (`jobs/jobs.go`):

1. Login: the callback enqueues `sync_installation` for every installation that GitHub lists for the user.
2. Webhooks: `installation`, `installation_repositories`, and `repository` events enqueue `sync_installation`. The handler does not trust payload contents for repository data. It re-reads GitHub.
3. Periodic: `reconcile_installations` runs every `RECONCILE_INTERVAL` (default `6h`, `config.go`) and at worker start (`RunOnStart: true`). It enqueues a sync for every installation that is not deleted and not suspended.

Sync algorithm (`SyncWorker.Work`):

- `GET /app/installations/{id}`. A 404 marks the installation deleted and revokes grants.
- Suspended installation: update the row, skip repositories.
- Record `started` in `sync_state`. Upsert each page with `synced_at = started`.
- After every page succeeded: tombstone rows with `synced_at < started`. A failed pass never tombstones.

## 8. Schema migration standards

**Verified**: goose migrations are embedded (`//go:embed migrations/*.sql`) and run by the `migrate` role. River migrations run after them.

**Decision** (rules for all new migrations):

1. File name `NNNNN_<topic>.sql`, sequential. One logical change per file.
2. Production is forward-only. A `-- +goose Down` section exists for local development only. Never run `down` against production. Recover with a new forward migration or a restore.
3. Expand and contract for breaking changes:
   - Expand: add nullable column or new table. Deploy code that writes both shapes.
   - Migrate data: a River job backfills in batches.
   - Contract: a later release drops the old column after no code reads it.
4. No long locks: create indexes on populated tables with `CREATE INDEX CONCURRENTLY` in a migration with `-- +goose NO TRANSACTION`. Add `NOT NULL` with a `CHECK … NOT VALID` then `VALIDATE CONSTRAINT`.
5. The `migrate` Job runs before the new `api` and `worker` pods start. Code must work with schema N and N+1 during a rollout.
6. CI runs all migrations on an empty PostgreSQL 18 database (`internal/testdb` creates one database per test). **Verified**: `core/internal/testdb/testdb.go` exists.

## 9. Job argument schema evolution

River stores job args as JSON in `river_job.args`. A job inserted by release N may run on release N+1. **Decision**:

1. Add fields only. New fields must have a safe zero value.
2. Never rename or remove a field while jobs of that kind can be pending. Drain or wait for retention first.
3. Never change the meaning of an existing field. Create a new kind (for example `sync_installation_v2`) and keep the old worker until no old rows remain.
4. Unique jobs dedupe by args (`ByArgs: true`). Adding a field changes the unique key. Treat that as a new kind if exact dedupe matters.
5. Job args hold ids only, never secrets or tokens. **Verified** for current kinds: `installation_id`, `delivery_id`, empty struct.

## 10. Planned tables (Stages 3 to 5)

All rows below are **Planned**. Column lists are proposals. Mapping from legacy tables is in `migration-plan.md`.

| Table | Class | Key | Source | Stage |
|---|---|---|---|---|
| `teams` | Mirror | `github_id` | `GET /orgs/{org}/teams` (`teams/list`) | 3 |
| `team_members` | Mirror | `(team_id, user_github_id)` | `GET /orgs/{org}/teams/{team_slug}/members` (`teams/list-members-in-org`) | 3 |
| `org_members` | Mirror | `(installation_id, user_github_id)` with `role` | `GET /orgs/{org}/members` (`orgs/list-members`) | 3 |
| `org_invitations` | Mirror | `github_id` | `GET /orgs/{org}/invitations` (`orgs/list-pending-invitations`) | 3 |
| `operations`, `operation_items` | Authoritative | `id`, `(operation_id, item_key)` | bulk operation plans and per-item results | 4 |
| `idempotency_keys` | Authoritative, ephemeral | `(user_id, key)` | `Idempotency-Key` header | 4 |
| `scoring_presets` + 7 child tables | Authoritative | `(installation_id, name)` | seed or legacy import | 5 |
| `signals` | Derived from GitHub events | dedup tuple `(user_id, type, repository_id, event_timestamp, content_hash)` | GraphQL ingest | 5 |
| `computed_scores`, `leaderboard_materializations` | Derived | see `database-audit.md` §7.2 | Rust `scoring` | 5 |
| `legacy_import_map` | Authoritative, audit | `(source_table, legacy_id)` | preset import | 5 |

**Decision**: `signals` starts unpartitioned. Revisit partitioning above about 10 M rows (`database-audit.md` §7.2). The Rust scoring service reads `signals` and writes `computed_scores`. It talks to core only through tables and job rows (decisions item 3).
