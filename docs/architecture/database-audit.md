# Database audit: legacy GH Org Tool data stores

Audit date: 2026-10-26. Scope: the Supabase PostgreSQL database and the Upstash Redis cache of the legacy app `~/dev/legacy/gh-org-tool` (read-only). This document gives the schema, the live data volume, a classification of each table and key namespace, and a migration plan to a self-hosted PostgreSQL on the k3s cluster.

Labels:

- **Verified**: a file path or a command proves the statement.
- **Assumption**: not proven. Check before you rely on it.
- **Decision**: a choice that this document makes for Gitorgasm.
- **Proposal**: a recommendation for the owner to accept or reject.

No secret values, personal data, or tokens appear in this document. Hostnames appear only as the first 8 hex characters of their SHA-256 hash.

## 1. Summary

- Only one of the three legacy Supabase projects is reachable. It is the project in `.env` (`NODE_ENV=development`). The staging and production projects return DNS `NXDOMAIN`. **Verified** (section 3).
- Both Upstash Redis databases (`.env` and `.env.staging`) return DNS `NXDOMAIN`. No live key data exists. The key namespaces below come from the code. **Verified** (section 5).
- The reachable database holds 949 rows in 25 exposed tables. The JSON export is 357 KiB. **Verified** (section 4).
- Only one group of data is application-owned and cannot come back from GitHub: the scoring presets (8 tables, 186 rows). All 6 presets are identical and unedited defaults. **Verified** (section 4.3).
- **Decision**: migrate the scoring presets. Rebuild all GitHub mirror data and all derived data from GitHub. Discard sessions, tokens, caches, locks, and sync metadata.

## 2. Schema (from migrations)

Source: `supabase/migrations/001_*.sql` to `011_*.sql` and `types/db/*.ts`. **Verified**.

### 2.1 Enums

| Enum                | Values                                                                                                       |
| ------------------- | ------------------------------------------------------------------------------------------------------------ |
| `account_type`      | organization, user                                                                                           |
| `signal_type`       | commit, pr_open, pr_merge, pr_close_no_merge, issue_open, issue_close, review, review_comment, comment, spam |
| `score_scope_type`  | organization, team, repository                                                                               |
| `score_entity_type` | contributor, team, repository                                                                                |
| `score_time_period` | today, week, month, quarter, half_year, all_time                                                             |
| `team_privacy`      | secret, closed, open                                                                                         |
| `review_state`      | approved, changes_requested, commented                                                                       |
| `multiplier_kind`   | merged_pr_commit, pr_linked_to_issue, first_activity                                                         |
| `zero_point_kind`   | self_review, self_merge, bot_activity, issue_closed_no_pr, pr_closed_no_merge                                |

### 2.2 Tables

PK = primary key. FK actions in brackets. All `created_at`/`updated_at` columns are `timestamptz NOT NULL DEFAULT now()`. Tables marked "upd trg" have a `BEFORE UPDATE` trigger that calls `set_updated_at()`.

| Table                                    | Key columns and types                                                                                                                                                                                                                                                                                                                               | PK                         | FKs                                                                | Other constraints and indexes                                                                                                                                             |
| ---------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------- | ------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `users`                                  | `id bigint` (GitHub user id), `login text`, `name text?`, `avatar_url text`, `is_bot bool`                                                                                                                                                                                                                                                          | id                         | none                                                               | unique `login`; partial index `is_bot=true`; upd trg                                                                                                                      |
| `organizations`                          | `id bigint` (GitHub org id), `login`, `name?`, `avatar_url?`                                                                                                                                                                                                                                                                                        | id                         | none                                                               | unique `login`; upd trg                                                                                                                                                   |
| `organization_memberships`               | `user_id`, `organization_id`, `role text` default `member`                                                                                                                                                                                                                                                                                          | (user_id, organization_id) | users, organizations [CASCADE]                                     | index per column                                                                                                                                                          |
| `repositories`                           | `id bigint` (GitHub repo id), `full_name`, `name`, `owner_login`, `organization_id?`, `is_private`, `is_archived`, `is_fork`, `default_branch?`, `html_url?`, `github_updated_at?`                                                                                                                                                                  | id                         | organizations [SET NULL]                                           | unique `full_name`; index `owner_login`; partial index `organization_id`                                                                                                  |
| `teams`                                  | `id bigint` (GitHub team id), `organization_id`, `slug`, `name`, `description?`, `privacy team_privacy`, `github_created_at?`, `github_updated_at?`                                                                                                                                                                                                 | id                         | organizations [CASCADE]                                            | unique (organization_id, slug); upd trg                                                                                                                                   |
| `team_members`                           | `team_id`, `user_id`, `role`                                                                                                                                                                                                                                                                                                                        | (team_id, user_id)         | teams, users [CASCADE]                                             | upd trg                                                                                                                                                                   |
| `team_repositories`                      | `team_id`, `repository_id`, `permission` default `push`                                                                                                                                                                                                                                                                                             | (team_id, repository_id)   | teams, repositories [CASCADE]                                      | upd trg                                                                                                                                                                   |
| `github_installations`                   | `installation_id bigint`, `app_id`, `organization_id?`, `user_account_id?`, `account_type`, `repository_selection`, `permissions jsonb`, `repository_count int`, `suspended_at?`, `suspended_by?`, `gh_created_at?`, `gh_updated_at?`                                                                                                               | installation_id            | organizations, users [CASCADE]                                     | CHECK exactly one of org/user set to match `account_type`; 4 indexes; upd trg                                                                                             |
| `installation_repositories`              | `installation_id`, `repository_id`                                                                                                                                                                                                                                                                                                                  | composite                  | github_installations, repositories [CASCADE]                       | index per column                                                                                                                                                          |
| `auth_sessions`                          | `id text` (session bearer token), `user_id`, `github_token_encrypted`, `_iv`, `_tag`, `github_token_type`, `github_token_scope`, `tokens_encrypted bool`, refresh token triple (nullable), `github_refresh_token_expires_at?`, `github_access_token_expires_at?`, `expires_at`                                                                      | id                         | users [CASCADE]                                                    | CHECK `tokens_encrypted = true`; index `user_id`, `expires_at`; upd trg                                                                                                   |
| `session_installations`                  | `session_id text`, `installation_id`                                                                                                                                                                                                                                                                                                                | composite                  | auth_sessions, github_installations [CASCADE]                      | index per column                                                                                                                                                          |
| `scoring_presets`                        | `id bigserial`, `installation_id`, `preset_name`, `is_active`                                                                                                                                                                                                                                                                                       | id                         | github_installations [CASCADE]                                     | unique (installation_id, preset_name); partial unique `installation_id WHERE is_active`; upd trg                                                                          |
| `scoring_preset_rules`                   | `preset_id`, `weekly_threshold int`=9, `decay_factor numeric(6,4)`=0.11, `floor_fraction numeric(6,4)`=0.2, `spam_penalty numeric(10,4)`=-12, `pr_closed_no_merge_penalty numeric(10,4)`=-10                                                                                                                                                        | preset_id (1:1)            | scoring_presets [CASCADE]                                          | upd trg                                                                                                                                                                   |
| `scoring_preset_base_points`             | `preset_id`, `signal_type`, `points numeric(10,4)`, `sort_order`                                                                                                                                                                                                                                                                                    | (preset_id, signal_type)   | scoring_presets [CASCADE]                                          | upd trg                                                                                                                                                                   |
| `scoring_preset_multipliers`             | `id bigserial`, `preset_id`, `kind multiplier_kind`, `factor numeric(10,4)`, `sort_order`                                                                                                                                                                                                                                                           | id                         | scoring_presets [CASCADE]                                          | unique (preset_id, kind); upd trg                                                                                                                                         |
| `scoring_preset_multiplier_signal_types` | `multiplier_id`, `signal_type`                                                                                                                                                                                                                                                                                                                      | composite                  | scoring_preset_multipliers [CASCADE]                               | index `signal_type`                                                                                                                                                       |
| `scoring_preset_review_state_weights`    | `preset_id`, `state review_state`, `weight numeric(10,4)`, `sort_order`                                                                                                                                                                                                                                                                             | (preset_id, state)         | scoring_presets [CASCADE]                                          | upd trg                                                                                                                                                                   |
| `scoring_preset_daily_quotas`            | `preset_id`, `signal_type`, `quota_limit int`, `sort_order`                                                                                                                                                                                                                                                                                         | (preset_id, signal_type)   | scoring_presets [CASCADE]                                          | upd trg                                                                                                                                                                   |
| `scoring_preset_zero_point_conditions`   | `preset_id`, `kind zero_point_kind`, `enabled bool`, `sort_order`                                                                                                                                                                                                                                                                                   | (preset_id, kind)          | scoring_presets [CASCADE]                                          | upd trg                                                                                                                                                                   |
| `signals`                                | `id bigint identity`, `installation_id`, `user_id`, `repository_id`, `type signal_type`, `value numeric(10,4)`, `event_timestamp`, `content_hash char(64)`, `metadata jsonb`                                                                                                                                                                        | (id, event_timestamp)      | github_installations [CASCADE]; users, repositories [RESTRICT]     | `PARTITION BY RANGE (event_timestamp)`; CHECK metadata ≤ 2048 bytes; unique dedup (user_id, type, repository_id, event_timestamp, content_hash); BRIN and 6 btree indexes |
| `signals_default`                        | partition of `signals`                                                                                                                                                                                                                                                                                                                              |                            |                                                                    | `DEFAULT` partition; `AFTER INSERT` trigger counts rows in `signals_partition_health`                                                                                     |
| `signals_partition_health`               | `singleton bool`, `default_partition_row_count bigint`, `last_default_partition_hit_at?`                                                                                                                                                                                                                                                            | singleton                  | none                                                               | one seeded row; own upd trg                                                                                                                                               |
| `ingest_log`                             | `id bigserial`, `installation_id`, `organization_id`, `team_id?`, `repository_id?`, `last_successful_ingest_at` default epoch, `ingest_preset text` default `all_time`, `repo_count`, `signal_count`                                                                                                                                                | id                         | installations, organizations, teams, repositories [CASCADE]        | CHECK at most one of team/repo; 3 partial unique indexes (org, team, repo scope); upd trg                                                                                 |
| `repository_sync_state`                  | `id bigserial`, `repository_id` unique, `organization_id`, `repo_updated_at?`, 5 `*_etag text?`, `pr_ids_hash?`, `last_fetched_at`                                                                                                                                                                                                                  | id                         | repositories, organizations [CASCADE]                              | index (organization_id, last_fetched_at)                                                                                                                                  |
| `leaderboard_materializations`           | `id bigserial`, `preset_id`, `scope_type`, `organization_id`, `team_id?`, `repository_id?`, `entity_type`, `time_period`, `version`, `row_count`, `signal_count`, `retention_state text`, `superseded_at?`, `retention_expires_at?`, `purged_at?`, `source_updated_at?`, `redis_cache_key text?`, `computed_at`, `signals_since?`, `signals_until?` | id                         | scoring_presets, organizations, teams, repositories [CASCADE]      | CHECK scope shape; CHECK retention_state in 4 values; CHECK lifecycle; 3 partial unique indexes; GC index; upd trg                                                        |
| `computed_scores`                        | `id bigserial`, `materialization_id`, `entity_type`, `user_id?`, `scored_team_id?`, `scored_repo_id?`, `time_period`, `score numeric(12,4)`, `breakdown jsonb`, `counts jsonb`, `additions`, `deletions`, `rank`, `member_count?`, `computed_at`, `version`                                                                                         | id                         | leaderboard_materializations, users, teams, repositories [CASCADE] | CHECK exactly one entity column; unique (materialization_id, entity_type, user_id, scored_team_id, scored_repo_id); 3 more indexes                                        |

### 2.3 Functions

| Function                                                         | Purpose                                                                                             | Called by app code?                |
| ---------------------------------------------------------------- | --------------------------------------------------------------------------------------------------- | ---------------------------------- |
| `set_updated_at()`                                               | trigger: `updated_at = now()`                                                                       | trigger only                       |
| `set_signals_partition_health_updated_at()`                      | same, for the health row                                                                            | trigger only                       |
| `track_signals_default_usage()`                                  | counts inserts into `signals_default`                                                               | trigger only                       |
| `ensure_signals_monthly_partitions(p_start, p_end)`              | creates `signals_YYYY_MM` partitions and 8 indexes each. `SECURITY DEFINER`, `search_path = public` | yes, `lib/supabase/signals.ts:211` |
| `drop_old_signals_partitions(p_keep_months=12)`                  | drops monthly partitions older than N months                                                        | no                                 |
| `purge_superseded_materializations(p_before)`                    | deletes non-current materializations                                                                | no                                 |
| `cleanup_expired_sessions()` and `purge_expired_auth_sessions()` | identical bodies: delete expired sessions                                                           | no                                 |

**Verified**: a grep for each function name in `lib/` and `app/` finds only the `ensure_signals_monthly_partitions` call. No `vercel.json` cron exists in the repo root. The three cleanup functions have no scheduler in this repo.

### 2.4 RLS, grants, policies

**Verified**: no `ENABLE ROW LEVEL SECURITY`, `POLICY`, or `GRANT` in `supabase/migrations/`. All access goes through the service-role key on the server (`createSupabaseAdminClient`). RLS is off on every table.

### 2.5 Schema findings

1. **Verified** (`011_computed_scores.sql`): the unique index `idx_cs_unique_entity` includes three nullable columns. Two of them are always NULL (CHECK `valid_score_entity`). Under the default `NULLS DISTINCT`, the index never rejects a duplicate. The app relies on a Redis lock instead (`lib/supabase/leaderboard-db.ts:1130`). Live data has 0 duplicates.
2. **Verified** (`lib/supabase/installation-repository.ts:116`): `upsertInstallationRepositories` has no caller. `installation_repositories` has 0 rows. The reads at `lib/auth/server/installation-service.ts:123,160` always return empty.
3. **Verified** (live data): all 17 `repository_sync_state` rows have NULL in every `*_etag` column and in `pr_ids_hash`. Only `repo_updated_at` and `last_fetched_at` hold data.
4. **Verified** (`004`, `005`, `007`): several single-column indexes duplicate the leading column of a composite PK (for example `idx_team_members_team`, `idx_team_repos_team`, `idx_installation_repos_installation`, `idx_session_installations_session`, `idx_scoring_preset_*_preset`). They cost write time and give no read benefit.
5. **Verified** (`009_sync_state.sql`): the parent `signals` table has 7 indexes, and each monthly partition gets 8 more by name. At 50 rows this is overhead with no benefit.

## 3. Environments and reachability

Method: parse only the needed keys from each env file in a Python process. Never print values. Hash hostnames with SHA-256. Resolve with the local resolver and with `dig @1.1.1.1`. **Verified**.

| Env file          | `NODE_ENV`  | Supabase host hash | Supabase DNS                         | Upstash host hash | Upstash DNS | Service-role key present |
| ----------------- | ----------- | ------------------ | ------------------------------------ | ----------------- | ----------- | ------------------------ |
| `.env`            | development | `f9b757c4`         | resolves, PostgREST HTTP 200/206     | `4df802c2`        | NXDOMAIN    | yes                      |
| `.env.staging`    | production  | `ed6414b8`         | NXDOMAIN (`URLError`, `gaierror -2`) | `7e42d517`        | NXDOMAIN    | yes                      |
| `.env.production` | production  | `2a24aaa2`         | NXDOMAIN                             | not set           | n/a         | no                       |

- The three files point at three **different** Supabase projects and two different Upstash databases. **Verified**.
- `.env` sets `NEXT_PUBLIC_APP_URL` to localhost. Staging points at an `upayan.dev` host. Production points at a `legacy-domain.com` host. **Verified**.
- **Assumption**: the staging and production Supabase projects and both Upstash databases were deleted. NXDOMAIN on a `*.supabase.co` or `*.upstash.io` name usually means the project no longer exists. A paused Supabase project normally still resolves. Ask the owner whether any other live project exists (for example env vars set only in the Vercel dashboard).
- **Decision**: treat the `.env` project as the only source for migration. It holds real installations (8) and organizations (10), with writes up to 2026-10-06.

## 4. Live data (Supabase project `f9b757c4`)

Method: `GET {url}/rest/v1/<table>?select=*&limit=0` with `Prefer: count=exact`. The row count is the total in `Content-Range`. Full rows were then exported with `Range` paging. **Verified**.

On-disk table sizes are **not reachable**. PostgREST does not expose `pg_total_relation_size`, and the env files hold no direct database connection string. The table below gives the JSON export size as a proxy.

### 4.1 Row counts

| Table                                  |                                                                                                                   Rows |                    JSON bytes |
| -------------------------------------- | ---------------------------------------------------------------------------------------------------------------------: | ----------------------------: |
| users                                  |                                                                                                                     29 |                         5,776 |
| organizations                          |                                                                                                                     10 |                         2,367 |
| organization_memberships               |                                                                                                                     16 |                         1,931 |
| repositories                           |                                                                                                                    372 |                       148,886 |
| teams                                  |                                                                                                                      7 |                         2,002 |
| team_members                           |                                                                                                                     25 |                         4,039 |
| team_repositories                      |                                                                                                                      4 |                           695 |
| github_installations                   |                                                                                                                      8 |                         6,430 |
| installation_repositories              |                                                                                                                      0 |                             2 |
| auth_sessions                          |                                                                                                                      2 | 778 (non-secret columns only) |
| session_installations                  |                                                                                                                      5 |  410 (`installation_id` only) |
| scoring_presets                        |                                                                                                                      6 |                         1,114 |
| scoring_preset_rules                   |                                                                                                                      6 |                         1,476 |
| scoring_preset_base_points             |                                                                                                                     42 |                         7,486 |
| scoring_preset_multipliers             |                                                                                                                     18 |                         3,369 |
| scoring_preset_multiplier_signal_types |                                                                                                                     54 |                         5,331 |
| scoring_preset_review_state_weights    |                                                                                                                     18 |                         3,144 |
| scoring_preset_daily_quotas            |                                                                                                                     12 |                         2,142 |
| scoring_preset_zero_point_conditions   |                                                                                                                     30 |                         5,334 |
| signals                                |                                                                                                                     50 |                        24,345 |
| signals_default                        | not reachable: PostgREST `404 PGRST205` (partitions are not in the schema cache). Rows are inside the `signals` count. |                               |
| signals_partition_health               |                                                                                                                      1 |                           194 |
| ingest_log                             |                                                                                                                      2 |                           671 |
| repository_sync_state                  |                                                                                                                     17 |                         5,925 |
| leaderboard_materializations           |                                                                                                                    106 |                        73,101 |
| computed_scores                        |                                                                                                                    109 |                        58,939 |
| **Total**                              |                                                                                                                **949** |                   **365,887** |

Schema drift: the PostgREST OpenAPI document (`GET /rest/v1/` with `Accept: application/openapi+json`) lists the same 25 tables. Every table has exactly the columns from the migrations. It also lists the 5 RPC functions from section 2.3. **Verified**. No drift.

### 4.2 Data shape observations

All **Verified** from the export.

- `signals`: types commit 40, pr_open 4, pr_merge 4, issue_open 1, issue_close 1. Event range 2024-06 to 2026-07. Months present: 2024-06, 2026-03 to 2026-07. Data covers 4 installations, 9 repositories, 5 users. `metadata` keys include `sha`, `message`, `title`, `body`, `additions`, `deletions`, `isBot`, `isInMergedPR`, `prNumber`, `linkedIssueIds`. Commit messages and PR bodies are user content.
- `signals_partition_health`: `default_partition_row_count = 0`. All signal rows landed in monthly partitions.
- `github_installations`: 8 rows, all `account_type = organization`, all `repository_selection = all`, none suspended.
- `leaderboard_materializations`: 106 rows, all `retention_state = current`. 52 have no `computed_scores` rows and no `signals_since`. Every row has a `redis_cache_key` of the form `leaderboard:snapshot:preset:<n>:org:<n>:entity:<type>:period:<p>:v:<n>`.
- `computed_scores`: 109 rows (team 48, contributor 32, repository 29). `breakdown` and `counts` keys: issues, commits, reviews, comments, pullRequests, reviewComments (`breakdown` also has `penalties`). No orphans. No duplicates.
- `repository_sync_state`: etag columns empty (section 2.5).
- `ingest_log`: 2 org-scope rows, `ingest_preset = month`.
- `auth_sessions`: 2 rows, both expired (2026-10-07).
- `users`: 2 bots. `repositories`: 31 private, 0 archived, 1 with NULL `organization_id`.
- No FK orphans between presets and installations or between memberships and users. 2 installations have no preset.

### 4.3 Scoring presets

**Verified**: 6 presets, one per installation, all named `Default`, all `is_active = true`. All 6 have `created_at = updated_at` in `scoring_presets` and `scoring_preset_rules`. The rule values are identical across all 6 and equal the column defaults (9, 0.11, 0.2, -12, -10). Base points, multipliers, review weights, quotas, and zero-point conditions each form exactly 1 distinct set across the 6 presets. Nobody customized a preset.

## 5. Redis (Upstash)

Live introspection is **not reachable**. Both REST hostnames return `NXDOMAIN` from the local resolver and from `1.1.1.1` (section 3). No command was sent. No DBSIZE, SCAN, TYPE, TTL, or memory data exists.

The namespaces below come from the code. **Verified** per file. Every key gets the prefix `${REDIS_CACHE_PREFIX}:` (default `gh-org-tool`, `lib/cache/redis.ts:43`, `lib/env/index.ts:18`). Both env files set the prefix to `gh-org-tool`. All values are JSON strings (`SET … EX`). The app uses only `GET`, `SET EX`, `SET NX EX`, `DEL`, and `SCAN`+`DEL` for pattern deletes.

| Namespace (after prefix)                                                                                                | Type              | TTL   | Written by                                | Content                   |
| ----------------------------------------------------------------------------------------------------------------------- | ----------------- | ----- | ----------------------------------------- | ------------------------- |
| `leaderboard:snapshot:preset:<presetId>:{org\|team\|repo}:<id>:entity:<entityType>:period:<period>:v:<version\|latest>` | string (JSON)     | 24 h  | `lib/leaderboard/redis-cache.ts:56`       | computed leaderboard rows |
| `leaderboard:response:<org>:<scopeType>:<scopeId>:<entityType>:<period>[:p<presetId>][:<from>][:<to>]`                  | string (JSON)     | 24 h  | `lib/leaderboard/request-cache.ts:67`     | full API response         |
| `leaderboard:lock:ingest:<installationId>:<org>:<scopeType>:<scopeId>`                                                  | string (ISO time) | 5 min | `lib/leaderboard/ingest.ts:145`           | ingest mutex              |
| `leaderboard:lock:scores:<presetId>:<scopeType>:<orgId>:<teamId\|null>:<repoId\|null>:<entityType>:<period>`            | string (ISO time) | 5 min | `lib/supabase/leaderboard-db.ts:1131`     | score write mutex         |
| `github:graphql:<queryName>:<var\|var\|…>`                                                                              | string (JSON)     | 90 s  | `lib/github/graphql-response-cache.ts:53` | GraphQL response          |
| `github:org-repositories:<org>`                                                                                         | string (JSON)     | 5 min | `lib/github/fetch-graphql.ts:192`         | org repo list             |
| `github:org-teams:v2:<org>`                                                                                             | string (JSON)     | 5 min | `lib/github/fetch-graphql.ts:985`         | team memberships          |
| `github:repo-fetch:id:<repoId>` and `github:repo-fetch:name:<owner/repo>`                                               | string (JSON)     | 24 h  | `lib/supabase/repo-cache.ts:48,54`        | repo sync-state copy      |

Every key has a TTL of 24 h or less. **Verified** (`lib/constants/time.ts:41-51`). Redis holds no data that is not in Postgres or GitHub.

## 6. Classification

Categories: GitHub-owned authoritative, Application-owned authoritative, Derived, Cache, Sync metadata, Audit, User preferences, Sensitive credentials, Temporary processing state.

| Item                                                            | Category                                       | Action      | Reason                                                                                                                                               |
| --------------------------------------------------------------- | ---------------------------------------------- | ----------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| `users`                                                         | GitHub-owned authoritative (mirror)            | Rebuild     | GitHub is the source. Login and org sync recreate rows.                                                                                              |
| `organizations`                                                 | GitHub-owned authoritative (mirror)            | Rebuild     | Created from installation and org sync.                                                                                                              |
| `organization_memberships`                                      | GitHub-owned authoritative (mirror)            | Rebuild     | Written at login (`lib/auth/server/user-service.ts:63`).                                                                                             |
| `repositories`                                                  | GitHub-owned authoritative (mirror)            | Rebuild     | GraphQL org sync.                                                                                                                                    |
| `teams`, `team_members`, `team_repositories`                    | GitHub-owned authoritative (mirror)            | Rebuild     | GraphQL team sync.                                                                                                                                   |
| `github_installations`                                          | GitHub-owned authoritative (mirror)            | Rebuild     | `GET /app/installations` with the App JWT lists all 8. Needed before preset import.                                                                  |
| `installation_repositories`                                     | GitHub-owned authoritative (mirror)            | Discard     | 0 rows. No writer (section 2.5).                                                                                                                     |
| `scoring_presets` + 7 child tables                              | Application-owned authoritative (org settings) | **Migrate** | The only data GitHub cannot recreate. 186 rows.                                                                                                      |
| `signals`                                                       | Derived (from GitHub events)                   | Rebuild     | Re-ingest with `all_time`. Use the dump only to reconcile.                                                                                           |
| `signals_default`, `signals_partition_health`                   | Sync metadata                                  | Discard     | Partition bookkeeping. Depends on the new partition design.                                                                                          |
| `leaderboard_materializations`                                  | Derived                                        | Rebuild     | Recomputed from signals and presets. Old ids and Redis keys become invalid.                                                                          |
| `computed_scores`                                               | Derived                                        | Rebuild     | Same as above.                                                                                                                                       |
| `ingest_log`                                                    | Sync metadata                                  | Discard     | Empty state forces a full first ingest, which is the goal.                                                                                           |
| `repository_sync_state`                                         | Sync metadata                                  | Discard     | Etags are empty. Timestamps only.                                                                                                                    |
| `auth_sessions`                                                 | Sensitive credentials                          | Discard     | Encrypted GitHub tokens bound to `TOKEN_ENCRYPTION_KEY`. Both rows expired. Users log in again.                                                      |
| `session_installations`                                         | Temporary processing state (session-bound)     | Discard     | Cascades from sessions.                                                                                                                              |
| Redis `leaderboard:snapshot:*`, `leaderboard:response:*`        | Cache                                          | Discard     | 24 h TTL. Rebuilt on read. Databases are gone anyway.                                                                                                |
| Redis `github:graphql:*`, `github:org-*`, `github:repo-fetch:*` | Cache                                          | Discard     | 90 s to 24 h TTL.                                                                                                                                    |
| Redis `leaderboard:lock:*`                                      | Temporary processing state                     | Discard     | 5 min mutex.                                                                                                                                         |
| Audit                                                           | none                                           | n/a         | No audit table exists. **Verified** (migrations).                                                                                                    |
| User preferences                                                | none server-side                               | n/a         | No table holds them. **Assumption**: UI settings (for example the sidebar cookie, `lib/constants/time.ts:15`) live in cookies or local storage only. |

**Decision**: the reason for not migrating sessions and tokens: the tokens are AES-256-GCM ciphertext (`lib/auth/server/crypto.ts:6`). Moving them would also mean moving the encryption key. Both sessions are expired. A new login costs users one OAuth round trip. No reason to keep them was found.

## 7. Migration plan

### 7.1 Target

**Decision**: one PostgreSQL 18 instance (`postgres:18-alpine`, **Verified** `deploy/kustomize/base/postgres.yaml`) in namespace `gitorgasm`, plain StatefulSet, as in `cluster-audit.md` section 6. CloudNativePG is not installed in the cluster and would add an operator to a node with about 79% of memory already requested. The data volume (under 1 MB) does not justify an operator. Supabase-only items (`anon`/`authenticated`/`service_role` roles, PostgREST, `auth` schema) do not carry over. **Planned**: the app connects with one least-privilege role. **Verified** current state: the manifests connect as `gitorgasm`, the user that `POSTGRES_USER` creates. The official image gives that user superuser power ([postgres image docs](https://hub.docker.com/_/postgres)). Risk R22 in `risk-register.md`.

### 7.2 Schema mapping (Proposals)

| Legacy                                                     | Gitorgasm                            | Change                                                                                                                                           |
| ---------------------------------------------------------- | ------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| all enums                                                  | same enums                           | Keep values. Add values with `ALTER TYPE … ADD VALUE` in later migrations.                                                                       |
| `users`, `organizations`, `teams`, `repositories`          | same tables                          | Keep GitHub numeric ids as `bigint` PKs. Add `node_id text` for GraphQL global ids.                                                              |
| `github_installations`                                     | `installations`                      | Same columns. Keep the account CHECK.                                                                                                            |
| `installation_repositories`                                | drop or keep with a real writer      | Decide in the app design.                                                                                                                        |
| `scoring_presets` + children                               | same structure                       | Use `GENERATED ALWAYS AS IDENTITY`. Keep `(installation_id, preset_name)` as the natural key. Keep the partial unique "one active preset" index. |
| `signals` (partitioned)                                    | `signals` (not partitioned)          | Data is 50 rows. Keep the dedup unique index and the BRIN index. Drop redundant btree indexes. Revisit partitioning above about 10 M rows.       |
| `computed_scores`                                          | same                                 | Make the entity unique index `NULLS NOT DISTINCT` (PG 15+) so the database enforces uniqueness.                                                  |
| `leaderboard_materializations`                             | same                                 | Remove `redis_cache_key` if the new cache derives keys.                                                                                          |
| `auth_sessions`, `session_installations`                   | new session design                   | No data carried over.                                                                                                                            |
| `ingest_log`, `repository_sync_state`                      | new sync-state table                 | Start empty.                                                                                                                                     |
| duplicate PK-prefix indexes                                | remove                               | Section 2.5 item 4.                                                                                                                              |
| `cleanup_expired_sessions` / `purge_expired_auth_sessions` | one function plus a scheduled caller | Legacy never called them.                                                                                                                        |

### 7.3 Transformation and idempotent import

Only scoring presets are imported. Steps:

1. Source: `~/.local/share/gitorgasm-dumps/supabase-dev-2026-10-26/scoring_preset*.json` (8 files) and `MANIFEST.json` (SHA-256 per file). Check the hashes first.
2. Prerequisite: the new app has synced installations from `GET /app/installations`. A preset whose `installation_id` is missing in the target is skipped and logged.
3. Map by natural key, not by legacy id. Presets: `(installation_id, preset_name)`. Children: `(preset, signal_type)`, `(preset, kind)`, `(preset, state)`. Multiplier signal types resolve through `(preset, kind)`.
4. Each preset with its children goes in one transaction with `INSERT … ON CONFLICT (natural key) DO UPDATE`. A re-run gives the same result.
5. Write the legacy-id to new-id map to a table `legacy_import_map(source_table, legacy_id, new_id, imported_at)` for audit and rollback.
6. **Proposal**: because all presets equal the defaults (section 4.3), an acceptable alternative is to seed a `Default` preset for every installation and skip the import. Use the import if the owner wants exact provenance.

GitHub mirror and derived data: trigger a full `all_time` ingest per installation after the presets exist. Then compute leaderboards.

### 7.4 Validation and reconciliation

| Check         | Pass condition                                                                                                                                                                                                    |
| ------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Preset count  | Target presets for imported installations = 6. Each installation has exactly 1 active preset.                                                                                                                     |
| Child counts  | 6 rules, 42 base points, 18 multipliers, 54 multiplier signal types, 18 review weights, 12 quotas, 30 zero-point rows.                                                                                            |
| Values        | Per-preset hash of child rows (ordered by natural key) equals the hash computed from the dump.                                                                                                                    |
| Installations | The 8 legacy installation ids exist, unless GitHub reports them uninstalled. List the difference.                                                                                                                 |
| Signals       | The legacy dedup tuple `(user_id, type, repository_id, event_timestamp, content_hash)` set from the dump is a subset of the rebuilt set. Explain each missing tuple (for example a deleted repo or a force-push). |
| Leaderboards  | For each org and period with legacy `computed_scores`, the top-10 contributor order matches, or the difference is explained by new signals after 2026-10-06.                                                      |

### 7.5 Rollback

- The legacy system has no live production database or cache (section 3). A rollback to it is not possible. **Verified**.
- Rollback inside Gitorgasm: delete imported presets by `legacy_import_map`, or restore the pre-import `pg_dump -Fc` taken just before the import. The cluster Restic job already backs up Postgres logical dumps to R2 (`cluster-audit.md`).
- Keep the JSON dumps until the reconciliation in 7.4 passes, plus 30 days. Then delete the dump directory.

### 7.6 Cutover

1. Deploy Gitorgasm with an empty database. Run migrations.
2. Register the GitHub App webhook and callback URLs for Gitorgasm.
3. Sync installations from GitHub.
4. Import presets (7.3). Run checks 7.4 rows 1-4.
5. Run full ingest and leaderboard compute. Run checks 7.4 rows 5-6.
6. Switch DNS or the Vercel frontend to the new API. Users log in again.

### 7.7 Post-cutover checks

- Every installation has 1 active preset.
- New signals arrive (row count grows after GitHub activity).
- The session cleanup and old-materialization purge jobs run on schedule and log row counts.
- Cache hit rate and lock contention are visible in Prometheus.
- A Restic backup of the new database exists in R2 and a restore drill succeeds.

## 8. Dumps

Location: `~/.local/share/gitorgasm-dumps/` (mode 700, files mode 600, outside any repository). **Verified** (`ls -la`).

- `supabase-dev-2026-10-26/<table>.json` for all 25 exposed tables (949 rows).
- `supabase-dev-2026-10-26/MANIFEST.json`: source host hash, export time, row count, byte size, and SHA-256 per file, and the list of excluded columns.
- Excluded: `auth_sessions.id` (session bearer token) and all `github_token_*`/`github_refresh_token_*` ciphertext, IV, and tag columns. `session_installations.session_id`. `signals_default` (not exposed).
- The dumps contain personal data (GitHub logins, names, avatars, commit messages). Do not commit or share them.
