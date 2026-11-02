# Workload model

This document estimates GitHub request counts, rate-budget use, database size, and worker concurrency for several scales. It names the dominant bottleneck for each scale. It lists the metrics that confirm or refute the estimates.

**All numbers in sections 2 to 5 are estimates** unless a row says **Verified**. No load test exists yet. Load tests are Planned for Stage 6.

## 1. Verified inputs

### 1.1 GitHub limits

| Limit                                              | Value                                                                                                          | Source                                                                                                                                                                                      |
| -------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Installation token, primary (non-Enterprise Cloud) | 5,000 req/h. Plus 50 req/h per repository above 20. Plus 50 req/h per org user above 20. Maximum 12,500 req/h. | **Verified**: [REST rate limits, GitHub App installations](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api#primary-rate-limit-for-github-app-installations) |
| Installation on Enterprise Cloud org               | 15,000 req/h                                                                                                   | same page                                                                                                                                                                                   |
| User access token (App on behalf of user)          | 5,000 req/h, shared with the user's other apps                                                                 | same page                                                                                                                                                                                   |
| GraphQL, installation                              | 5,000 points/h, same +50 per repo/user scaling, maximum 12,500 points/h                                        | **Verified**: [GraphQL rate limits](https://docs.github.com/en/graphql/overview/rate-limits-and-query-limits-for-the-graphql-api#primary-rate-limit)                                        |
| Secondary: concurrency                             | No more than 100 concurrent requests, shared by REST and GraphQL                                               | **Verified**: [REST secondary rate limits](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api#about-secondary-rate-limits)                                     |
| Secondary: points per minute                       | 900 points/min for REST, 2,000 points/min for GraphQL                                                          | same page                                                                                                                                                                                   |
| Secondary: CPU time                                | 90 s of CPU per 60 s real time (60 s for GraphQL)                                                              | same page                                                                                                                                                                                   |
| Secondary: content creation                        | About 80 content-generating requests/min and 500/h                                                             | same page                                                                                                                                                                                   |
| Org invitations                                    | 50 per org per 24 h. 500 per 24 h if the org is older than one month or on a paid plan.                        | **Verified**: [REST API: organization members](https://docs.github.com/en/rest/orgs/members)                                                                                                |
| Webhook response time                              | 2XX within 10 s, else GitHub counts a failure                                                                  | **Verified**: [Webhook best practices](https://docs.github.com/en/webhooks/using-webhooks/best-practices-for-using-webhooks#respond-within-10-seconds)                                      |
| Rate-limit response                                | 403 or 429. Wait for `retry-after`, else `x-ratelimit-reset`, else at least 1 min.                             | **Verified**: REST rate limits page, "Exceeding the rate limit"                                                                                                                             |

**Verified** consequence: each installation has its own primary budget. More orgs add more budget. They do not share it.

**Assumption**: the requests made with the App JWT (`GET /app/installations/{id}`, `POST /app/installations/{id}/access_tokens`) do not count against an installation's budget. The docs page above does not state the JWT budget. Gitorgasm caches installation tokens until 5 min before expiry (**Verified**: `core/internal/github/tokens.go`, `installationToken`), so JWT calls are at most about 1 per installation per hour plus 1 per sync.

### 1.2 Current code behavior

| Fact                                                                                                                                                                                                                                          | Source                                                                          |
| --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------- |
| Repository sync lists `GET /installation/repositories?per_page=100` and follows `Link` pages.                                                                                                                                                 | **Verified**: `core/internal/github/installations.go` (`ListInstallationRepositories`) |
| One full sync = 1 `GET /app/installations/{id}` (JWT), up to one token mint (JWT), and `ceil(repos/100)` pages (installation token). | **Verified**: `core/internal/jobs/sync` |
| Login = token exchange on `github.com` + `GET /user` + `ceil(installations/100)` pages of `GET /user/installations` (user token). It enqueues one sync per installation.                                                                      | **Verified**: `core/internal/httpapi/auth/oauth.go` (`Callback`)                      |
| Reconcile enqueues one sync per active installation every `RECONCILE_INTERVAL` (default `6h`), also on worker start. | **Verified**: `internal/jobs/maintenance`, `core/internal/config/config.go` |
| Worker concurrency: `WORKER_CONCURRENCY` default 4, applied to both the `default` and `github` queues per replica.                                                                                                                            | **Verified**: `config.go`, `jobs.NewWorkerClient`                               |
| Database pool: `DB_MAX_CONNS` default 10 per process in code, 8 in the production ConfigMap.                                                                                                                                                  | **Verified**: `config.go`, `deploy/kustomize/base/config.env`                  |
| Webhook events that cause work: `installation`, `installation_repositories`, and `repository` enqueue a full installation sync. Revoked `github_app_authorization` removes sessions and grants. | **Verified**: `internal/jobs/webhook`, `store/identity.RevokeGitHubUser` |
| Rate-limited sync snoozes until the retry time. Permission errors cancel. | **Verified**: `internal/jobs/internal/execution` |

### 1.3 Sync deduplication and serialization

**Verified** (`core/internal/jobs/queue`): `queue.SyncInstallation.InsertOpts` sets `UniqueOpts{ByArgs: true, ByPeriod: time.Minute}`. River keeps at most one `sync_installation` job per installation per 1-minute window. A webhook that arrives during a running sync falls into a later window and queues one follow-up pass. The sync worker in `core/internal/jobs/sync` takes a session advisory lock `pg_try_advisory_lock(hashtextextended('gitorgasm:sync_installation:' || installation_id, 0))` for the whole pass (`execution.WithSessionLock`). If another pass holds the lock, the job snoozes 30 s. So at most one pass runs per installation at a time.

**Verified** (River source): without `ByPeriod`, the default unique states include `completed`, and completed jobs stay for 24 h (`river@v0.49.0/insert_opts.go`, `rivershared@v0.49.0/riversharedmaintenance`). The code comment on `SyncInstallation.InsertOpts` in `jobs/queue` records why `ByPeriod` is set.

Consequence for the numbers below: a burst of N sync triggers over T minutes costs at most about T + 1 passes per installation, not N.

Note: the lock key is a 64-bit hash of a namespaced string (`syncLockKey` in `jobs/sync`). The installation id is not truncated, and the namespace keeps the key apart from goose and River locks. Two installations share a key only on a 64-bit hash collision. The effect would be extra snoozes, not data corruption.

## 2. Scenarios: repository mirror (implemented path)

Per installation. "Pass" = one full sync.

| Scenario              | Installation budget (req/h) | Requests per pass | Passes/day from reconcile | Daily requests (reconcile only) | Budget per pass |
| --------------------- | --------------------------- | ----------------- | ------------------------- | ------------------------------- | --------------- |
| 10 repos              | 5,000                       | 1                 | 4                         | 4                               | 0.02%           |
| 100 repos (≤20 users) | 5,000 + 80 × 50 = 9,000     | 1                 | 4                         | 4                               | 0.01%           |
| 1,000 repos           | 12,500 (cap)                | 10                | 4                         | 40                              | 0.08%           |
| 10,000 repos          | 12,500 (cap)                | 100               | 4                         | 400                             | 0.8%            |

The budget formula is **Verified** (section 1.1). The rows are arithmetic on it. JWT requests are excluded (section 1.1 assumption).

**Estimate**: one page takes about 100–500 ms of network time (**Assumption**, as in `tests/performance/lang-bench/RESULTS.md`). Go parses a 100-repo page in about 0.5 ms (**Verified** scaled from RESULTS.md: 10,000 repos in 40–60 ms). A 10,000-repo pass is therefore about 10–50 s of network wait and under 0.1 s of CPU. The 15 min job timeout (`Worker.Timeout` in `jobs/sync`) has wide margin.

### 2.1 Multiple orgs

| Scenario                                    | Effect                                                                                                                                 |
| ------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------- |
| 10 orgs × 372 repos total (legacy dev data) | 10 independent budgets. About 10–14 requests per reconcile round in total.                                                             |
| 100 orgs × 100 repos                        | 100 syncs per reconcile round, one page each. 4 workers process them in about 100 × 0.5 s / 4 ≈ 13 s. **Estimate**.                    |
| 1,000 orgs × 1,000 repos                    | 1,000 syncs × 10 pages. With 4 workers: about 1,000 × 10 × 0.3 s / 4 ≈ 750 s per round. **Estimate**. Still far inside a 6 h interval. |

**Decision**: the reconcile round causes a burst of `InsertMany` with one job per installation. At 1,000 installations this is one insert batch of 1,000 rows. Postgres handles this easily (**Assumption**, not measured).

### 2.2 Concurrent admins

Logins cost the **user** budget (5,000 req/h per user, shared with other apps the user uses). Each login costs 2 user-token requests for users with ≤100 installations. Read endpoints call no GitHub API. **Verified**: `httpapi/organization` list handlers read only Postgres.

| Scenario                                | GitHub cost                    | Database cost                                                                                                               |
| --------------------------------------- | ------------------------------ | --------------------------------------------------------------------------------------------------------------------------- |
| 10 concurrent admins browsing           | 0 req (reads), 2 req per login | Each request: 1 session lookup + 1–2 queries.                                                                               |
| 100 concurrent admins                   | Same per user                  | ≤ `DB_MAX_CONNS` pool connections per api replica (8 in production). **Estimate**: pool queueing appears before CPU limits. |
| 100 admins press "sync" on the same org | 1 sync after dedupe            | 100 job insert attempts, 1 kept, plus 100 audit rows.                                                                       |

### 2.3 Webhook bursts

Each delivery creates one `webhook_deliveries` row and one River job row in one transaction.
One `process_webhook` job then runs. **Verified**: `httpapi/webhook/deliveries.go` and `internal/jobs/webhook`.

| Burst                                            | GitHub cost (1-minute dedupe) | DB writes                                | Bottleneck                                                                                                          |
| ------------------------------------------------ | ----------------------------- | ---------------------------------------- | ------------------------------------------------------------------------------------------------------------------- |
| 100 `repository` events in 1 min, 1,000-repo org | 1–2 syncs = 10–20 req         | ~200 rows inserted, ~100 job completions | none                                                                                                                |
| 1,000 events in 1 min, 10,000-repo org           | 1–2 syncs = 100–200 req       | ~2,000 rows                              | Webhook worker throughput (4 workers per replica, `default` queue). **Estimate**: hundreds of jobs/s. Not measured. |
| 1,000 events without dedupe (counterfactual)     | 1,000 × 100 = 100,000 req     | same                                     | Primary budget exhausted in under an hour. This is why dedupe matters.                                              |

**Estimate**: payload size is a few KB to tens of KB per delivery (**Assumption**; GitHub caps payloads at 25 MB). At 10,000 deliveries/day × 20 KB ≈ 200 MB/day. `purge_webhook_deliveries` deletes processed deliveries after 14 days and all deliveries after 30 days, so the table holds about 14 days of payloads (≈ 2.8 GB at the upper rate).

### 2.4 Parallel sync

The `github` queue has `MaxWorkers = WORKER_CONCURRENCY` (default 4) per worker replica (**Verified**: `jobs.NewWorkerClient`). Each sync is sequential inside (pages one after another). So total in-flight GitHub requests ≤ 4 × replicas. GitHub allows 100 concurrent requests (section 1.1). **Decision**: keep total GitHub concurrency ≤ 20 until per-installation budgets exist (Planned, see [scalability.md](./scalability.md)).

## 3. Scenarios: planned features

These paths do not exist in code. Endpoints are **Verified** to exist in the pinned REST description (`github/rest-api-description` v2.1.0, `api.github.com.json`): `/orgs/{org}/members`, `/orgs/{org}/invitations`, `/orgs/{org}/teams`, `/orgs/{org}/teams/{team_slug}/members`, `/orgs/{org}/teams/{team_slug}/repos`, `/orgs/{org}/memberships/{username}`, `/orgs/{org}/teams/{team_slug}/memberships/{username}`, `/user/memberships/orgs/{org}`.

### 3.1 Membership and team mirror (Stage 3)

Per pass: `ceil(members/100)` + `ceil(invitations/100)` + `ceil(teams/100)` + Σ per team (`ceil(team_members/100)` + `ceil(team_repos/100)`).

| Org size                               | Requests per pass (estimate)       | Budget per pass |
| -------------------------------------- | ---------------------------------- | --------------- |
| 30 members, 5 teams, 100 repos         | 1 + 1 + 1 + 5 × 2 = 13             | 0.14% of 9,000  |
| 500 members, 50 teams, 1,000 repos     | 5 + 1 + 1 + 50 × (1 + 1) = 107     | 0.9% of 12,500  |
| 5,000 members, 500 teams, 10,000 repos | 50 + 5 + 5 + 500 × (1 + 2) = 1,560 | 12.5% of 12,500 |

### 3.2 Large bulk mutations (Stage 4)

Each item = 1 mutation request. Role change also needs 1 membership check. Admin re-check before the job = 1 user-token request (`GET /user/memberships/orgs/{org}`).

| Operation        | Items | Requests | Dominant limit                                                                  | Minimum wall time               |
| ---------------- | ----- | -------- | ------------------------------------------------------------------------------- | ------------------------------- |
| Mass invite      | 50    | 50       | Invitation cap 50 or 500 per 24 h                                               | < 1 min                         |
| Mass invite      | 500   | 500      | Invitation cap 500 per 24 h. Content limit 80/min, 500/h.                       | ≥ 7 min (80/min)                |
| Mass invite      | 1,000 | 1,000    | Invitation cap: 500 per 24 h                                                    | ≥ 24 h. Must split across days. |
| Bulk role change | 1,000 | 2,000    | Content limit 500/h (**Assumption**: membership PUT counts as content creation) | ≥ 2 h                           |
| Team bulk-add    | 1,000 | 1,000    | Content limit 500/h (same assumption)                                           | ≥ 2 h                           |

**Decision**: bulk jobs must pace mutations below 80/min and checkpoint each item. They must stop and report when the invitation cap is reached, not retry. This is why bulk operations are River jobs with per-item results (D5, Stage 4).

### 3.3 Leaderboard ingest (Stage 5)

Legacy runs 4 GraphQL query families per repository with up to 20 pages each (**Verified**: legacy-audit §5.3, §5.4). Each GraphQL request costs at least 1 point (**Verified**: GraphQL rate limits page, "Calculating the primary rate limit points").

| Scale        | Backfill queries (estimate: 4 families × 2 pages × repos) | Points (≥) | Backfill time at the cap |
| ------------ | --------------------------------------------------------- | ---------- | ------------------------ |
| 100 repos    | 800                                                       | 800        | < 1 h at 9,000/h         |
| 1,000 repos  | 8,000                                                     | 8,000      | about 1 h at 12,500/h    |
| 10,000 repos | 80,000                                                    | 80,000     | ≥ 6.4 h at 12,500/h      |

**Estimate**: ingest backfill is the largest consumer of GitHub budget in any planned feature. **Decision**: ingest must use per-repo cursors, cost accounting from the `rateLimit` GraphQL field, and webhooks for incremental updates (Stage 5).

## 4. Database size estimates

Row sizes are **Assumption** (column widths in `core/internal/db/migrations/` plus about 30% index overhead).

| Table                                     | Row size (est.)                          | 10 repos                           | 1,000 repos                       | 10,000 repos × 10 orgs                   |
| ----------------------------------------- | ---------------------------------------- | ---------------------------------- | --------------------------------- | ---------------------------------------- |
| `repositories`                            | ~0.5 KB + index                          | 5 KB                               | 0.5 MB                            | 50 MB                                    |
| `installations`, `accounts`, `sync_state` | ~0.3 KB                                  | negligible                         | negligible                        | negligible                               |
| `users`, `sessions`, `user_installations` | ~0.3 KB                                  | negligible                         | < 1 MB at 1,000 users             | < 5 MB                                   |
| `webhook_deliveries`                      | ~20 KB                                   | grows with events                  | 10–200 MB/day                     | bounded at ~14 days (daily purge)        |
| `river_job`                               | ~1 KB                                    | cleaned after 24 h (River default) | bounded by daily jobs             | bounded                                  |
| `audit_events`                            | ~0.5 KB                                  | negligible                         | grows with actions                | grows with actions                       |
| `signals` (Stage 5)                       | ~0.5–2 KB (metadata ≤ 2,048 B in legacy) | —                                  | 1–10 M rows/year (**Assumption**) | 10–100 M rows/year → partitioning needed |

**Verified** reference point: legacy dev Supabase holds 949 rows in 357 KiB of JSON ([database-audit.md](./database-audit.md)).

**Estimate**: below Stage 5, the database stays under 1 GB if webhook payloads are purged. The Postgres pod request of 256Mi is enough for this working set (**Assumption**).

## 5. Dominant bottleneck per scale

| Scale                         | Dominant bottleneck                                                                    | Why                                                                      |
| ----------------------------- | -------------------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| 10 repos, 1 org               | None                                                                                   | Everything is idle. Fixed cost (pod memory) dominates.                   |
| 100 repos, ~10 orgs           | Cluster memory headroom                                                                | Requests ~512Mi of ~1,508Mi free request headroom (cluster-audit.md §2). |
| 1,000 repos                   | GitHub secondary limits during bulk operations and ingest                              | Content creation 80/min and invitation caps.                             |
| 10,000+ repos                 | GitHub primary budget for ingest (GraphQL points); Postgres disk and I/O for `signals` | Section 3.3 and section 4.                                               |
| Any scale with webhook bursts | Sync dedupe window                                                                     | Section 1.3. One pass per installation per minute of burst.              |

CPU is not a bottleneck at any listed scale (**Verified** micro-benchmarks in `tests/performance/lang-bench/RESULTS.md`; extrapolation is an **Assumption**).

## 6. Key metrics

| Metric                                       | How measured now                                                                                                                     | Status                                                                                             |
| -------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------- |
| API latency p50/p95/p99 per route            | `histogram_quantile` over `gitorgasm_http_request_duration_seconds{route}` (buckets 5 ms to 5 s)                                     | **Verified**: `core/internal/httpapi/middleware`                                                   |
| API request rate and error rate              | `gitorgasm_http_requests_total{route,method,status}`                                                                                 | **Verified**: `httpapi/middleware`                                                                 |
| GitHub request rate and errors per operation | `gitorgasm_github_requests_total{operation,status}`                                                                                  | **Verified**: `core/internal/github/transport.go`                                                  |
| Rate budget remaining                        | `gitorgasm_github_rate_limit_remaining{credential}` from `x-ratelimit-remaining`                                                     | **Verified**: `github/transport.go`. Per-installation labels: check label cardinality before Stage 3. |
| Rate budget reset time and used fraction     | Not exported                                                                                                                         | Planned: gauge from `x-ratelimit-reset` and `x-ratelimit-limit`                                    |
| Queue depth per queue and state              | Not exported. Manual: `SELECT queue, state, count(*) FROM river_job GROUP BY 1,2`                                                    | Planned: River event subscription or SQL exporter                                                  |
| Queue age (oldest available job)             | Manual: `SELECT queue, now() - min(scheduled_at) FROM river_job WHERE state = 'available' GROUP BY 1`                                | Planned metric                                                                                     |
| Job duration and outcome per kind            | Not exported. River logs through `slog` (`Logger` in `jobs.NewWorkerClient`).                                                        | Planned (D14)                                                                                      |
| Sync freshness per installation              | Manual: `now() - sync_state.last_succeeded_at`                                                                                       | Planned gauge                                                                                      |
| Webhook backlog                              | Manual: `SELECT count(*) FROM webhook_deliveries WHERE processed_at IS NULL` (partial index `webhook_deliveries_unprocessed_idx`)    | Planned gauge                                                                                      |
| Webhook ack latency                          | `gitorgasm_http_request_duration_seconds{route="POST /webhooks/github"}` (**Assumption**: route label equals the mux pattern)        | **Verified** histogram exists                                                                      |
| DB pool saturation                           | Not exported (`pgxpool.Stat()` is available in pgx)                                                                                  | Planned                                                                                            |
| DB size per table                            | Manual: `pg_total_relation_size`                                                                                                     | Planned                                                                                            |
| Process memory and GC                        | Go runtime collectors in the default Prometheus registry (**Assumption**: `promauto` uses the default registry, which includes them) | **Verified** registry use in code                                                                  |
| Bulk operation progress                      | —                                                                                                                                    | Planned (Stage 4, SSE)                                                                             |
