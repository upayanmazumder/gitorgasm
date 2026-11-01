# API design

Status: Stage 1 (Foundation). Contract: `contracts/openapi/v1.yaml` (OpenAPI 3.0.3). Implementation: `core/internal/httpapi`.

Labels: **Verified** (file path or command), **Assumption**, **Decision**, **Alternative considered**, **Planned**.

## 1. Principles

**Decision** (decisions item 6):

1. Public API = REST + JSON over HTTPS. No public GraphQL API.
2. The OpenAPI file is the contract. Code follows the contract. A contract change and a code change ship in the same pull request.
3. Tests validate real responses against the contract. **Verified**: `core/internal/httpapi/api_test.go` imports `kin-openapi/openapi3filter`.
4. Resources are nouns under `/v1`. Organizations are addressed by GitHub login (`/v1/organizations/{login}`). Login match is case-insensitive (`lower(a.login) = lower($2)` in `store.AccessibleInstallation`).
5. Inaccessible and unknown organizations both return 404 `organization_not_found`. This prevents tenant enumeration. **Verified**: `httpapi/orgs.go` `resolveOrg`.
6. Long work is asynchronous. The API enqueues a job and returns `202`. **Verified**: `POST /v1/organizations/{login}/sync`.
7. Field names are `snake_case`. Timestamps are RFC 3339 UTC strings. GitHub ids are `int64` and named `github_id`. Internal ids are not exposed (`SessionUser.UserID` has `json:"-"`).

**Alternative considered**: GraphQL for the frontend. Rejected: extra attack surface (query cost, introspection), and the UI needs a small fixed set of views.

## 2. Endpoints (current)

**Verified**: routes in `httpapi/server.go` `Handler`.

| Method and path                              | Auth              | Success   | Purpose                            |
| -------------------------------------------- | ----------------- | --------- | ---------------------------------- |
| `GET /healthz`                               | none              | 200       | liveness, no dependencies          |
| `GET /readyz`                                | none              | 200 / 503 | ready only when PostgreSQL answers |
| `GET /v1/auth/login`                         | none              | 302       | sign in / refresh grants / GitHub Setup URL |
| `GET /v1/auth/callback`                      | none              | 302       | refresh grants, create session, redirect to `/organizations` |
| `GET /v1/auth/install`                       | none              | 302       | redirect to App install page       |
| `POST /v1/auth/logout`                       | cookie (optional) | 204       | delete session and clear pending OAuth |
| `GET /v1/me`                                 | session           | 200       | current user                       |
| `GET /v1/organizations`                      | session           | 200       | accessible organizations           |
| `GET /v1/organizations/{login}/repositories` | session           | 200       | repositories, keyset paginated     |
| `POST /v1/organizations/{login}/sync`        | session           | 202       | queue a repository sync            |
| `POST /webhooks/github`                      | HMAC              | 202       | GitHub webhook inbox               |

## 3. Error contract

**Verified**: one body shape for every error (`httpapi/response`, schema `Error` in the contract):

```json
{
  "error": {
    "code": "organization_not_found",
    "message": "organization not found",
    "request_id": "…"
  }
}
```

- `code` is stable and machine-readable. Clients branch on `code`, never on `message`.
- `message` is for humans. It never contains upstream GitHub bodies (`githubError` maps upstream failures to fixed messages).
- `request_id` equals the `X-Request-Id` response header. An inbound `X-Request-Id` that matches `^[A-Za-z0-9._-]{8,64}$` is reused. Else `middleware.RequestID` generates one.

Codes in use. **Verified** in HTTP handlers and `httpapi/response`:

| Code                     | HTTP                | Where                        |
| ------------------------ | ------------------- | ---------------------------- |
| `internal`               | 500                 | `response.InternalError`, `middleware.Recoverer` |
| `github_not_configured`  | 503                 | `requireGitHub`              |
| `origin_rejected`        | 403                 | `middleware.CheckOrigin`                |
| `unauthenticated`        | 401                 | `authed`                     |
| `oauth_state_missing`    | 400                 | `callback`                   |
| `oauth_state_mismatch`   | 400                 | `callback`                   |
| `oauth_denied`           | 400                 | `callback`                   |
| `github_rate_limited`    | 503 + `Retry-After` | `githubError`                |
| `github_auth_failed`     | 400                 | `githubError`                |
| `github_unavailable`     | 502                 | `githubError`                |
| `organization_not_found` | 404                 | `resolveOrg`                 |
| `invalid_limit`          | 400                 | `listRepositories`           |
| `invalid_cursor`         | 400                 | `listRepositories`           |
| `payload_too_large`      | 413                 | `webhook`                    |
| `invalid_signature`      | 401                 | `webhook`                    |
| `invalid_delivery`       | 400                 | `webhook`                    |
| `invalid_payload`        | 400                 | `webhook`                    |

**Decision** (rules for new codes): `snake_case`, noun first where possible, one code per distinct client action. Add the code to this table in the same pull request.

Known contract gaps (**Verified** by comparing the contract with code):

- Closed: every non-probe operation in `v1.yaml` now has a shared `default` response that references `#/components/responses/Error` (**Verified** `contracts/openapi/v1.yaml`). This covers 500 `internal`, 503 `github_not_configured`, and other error codes that a path does not list explicitly.
- `GET /v1/auth/callback` returns JSON errors to the browser. **Planned** (Stage 2): redirect to a frontend error page with the `code` as a query parameter.

## 4. Pagination

**Verified** (`httpapi/orgs.go`, `store.ListRepositories`):

- Keyset pagination. Response shape: `{"data": [...], "next_cursor": "<opaque>" | null}`.
- `limit` default 50, range 1 to 100. Out of range returns 400 `invalid_limit`.
- `cursor` is base64url JSON of the last key `(lower(name), github_id)`. Clients must treat it as opaque. Malformed returns 400 `invalid_cursor`.
- The server fetches `limit + 1` rows to know if another page exists. No `COUNT(*)` per page.
- The index `repositories_listing_idx (installation_id, lower(name), github_id) WHERE deleted_at IS NULL` serves the query.
- `GET /v1/organizations` returns the same envelope. `next_cursor` is always `null` now, because the list is small. The envelope lets pagination arrive later without a breaking change.

**Decision**: no offset pagination. Offsets drift under concurrent sync writes and cost O(offset).

**Planned**: sign or MAC the cursor if it ever carries data that the client must not change. The current cursor carries only a sort key. A forged cursor can only skip rows inside the caller's own tenant.

## 5. Filtering and sorting (Planned)

Conventions for Stage 2 and later. Nothing below is implemented.

- Filters are query parameters named after the field: `?archived=false&visibility=private`.
- Text search: `?q=` with PostgreSQL `pg_trgm` (decisions item 4).
- Sorting: `?sort=name` or `?sort=-pushed_at`. Each sortable field needs a matching keyset index. The cursor encodes the sort key, so a cursor is valid only with the same `sort`.
- Unknown filter or sort values return 400 `invalid_filter` / `invalid_sort`. The server never ignores an unknown parameter silently.

## 6. Versioning

- **Verified**: path version `/v1`.
- **Decision**: additive changes (new fields, new endpoints, new optional parameters, new error codes) stay in `v1`. Clients must ignore unknown fields.
- A breaking change (remove or rename a field, change a type, change meaning) needs `/v2` for the affected resources. Run `v1` and `v2` in parallel until the frontend migrates.
- The frontend is the only client now. **Decision**: deprecation window = one frontend release after `v2` ships.

## 7. Validation

**Verified** now:

- Path and query parameters are parsed and range-checked in handlers (`limit`, `cursor`).
- Webhook body: size cap, signature, required headers, JSON parse.
- Response shapes are validated against the contract in tests (`additionalProperties: false` on response schemas).

**Decision** for request bodies (Stage 4): decode with `json.Decoder.DisallowUnknownFields`, cap body size per route, validate GitHub logins with GitHub's login rules before any GitHub call, and return 400 `invalid_request` with a `details` array of `{field, problem}`. **Planned**: add `details` as an optional property of `Error` in `v1` (additive).

## 8. Idempotency

**Verified** now:

- `POST …/sync`: River unique job by args. A repeated request while a sync is pending returns the same `job_id` with `duplicate: true`.
- `POST /webhooks/github`: primary key on `X-GitHub-Delivery`.
- `POST /v1/auth/logout`: deleting a missing session is a no-op.

**Planned** (Stage 4): `Idempotency-Key` request header on every mutation that creates an operation.

- Key scope: `(user_id, key)`. Store request hash, status code, and response body in `idempotency_keys` for 24 hours.
- Same key + same request hash: return the stored response.
- Same key + different request hash: 422 `idempotency_key_reused`.
- Same key while the first request runs: 409 `idempotency_in_progress`.

## 9. Operations, status, and cancellation (Planned, Stage 4)

Design for bulk mutations. The flow is in `github-integration.md` §8.

| Method and path                                          | Purpose                                                            |
| -------------------------------------------------------- | ------------------------------------------------------------------ |
| `POST /v1/organizations/{login}/operations`              | create a plan (kind, items). Returns 201 `{id, status: "planned"}` |
| `POST /v1/organizations/{login}/operations/{id}/dry-run` | run read-only checks. 202                                          |
| `POST /v1/organizations/{login}/operations/{id}/execute` | re-check admin role, start. 202                                    |
| `POST /v1/organizations/{login}/operations/{id}/cancel`  | stop pending items. 202                                            |
| `GET /v1/organizations/{login}/operations/{id}`          | status and counts                                                  |
| `GET /v1/organizations/{login}/operations/{id}/items`    | per-item results, keyset paginated                                 |
| `GET /v1/organizations/{login}/operations/{id}/events`   | SSE progress                                                       |

Status values: `planned`, `dry_run`, `ready`, `running`, `cancelling`, `completed`, `partially_failed`, `failed`, `cancelled`.

**Planned**: a generic `GET /v1/jobs/{id}` is not exposed. River job ids are internal. The sync endpoint returns `job_id` today. **Decision**: keep it for now. Replace it with an operation id when operations exist (additive: add `operation_id`, deprecate `job_id` in `v2`).

## 10. Server-Sent Events (Planned, Stage 4)

- `Content-Type: text/event-stream`. Events: `progress` (`{done, failed, total}`), `item` (`{item_key, status, code}`), `end` (`{status}`).
- Each event has an `id`. The client resumes with `Last-Event-ID`.
- Source of truth stays in PostgreSQL. The SSE handler polls the operation row each second, or uses `LISTEN/NOTIFY`. No in-memory fan-out, so any api replica can serve the stream.
- **Assumption**: Cloudflare and Vercel rewrites buffer or time out long responses. Verify streaming through the Vercel `/api` rewrite before shipping. Fallback: client polling of `GET …/operations/{id}`.

## 11. TypeScript client (Planned)

**Decision**: generate types from `contracts/openapi/v1.yaml` with `openapi-typescript`. Use `openapi-fetch` or a thin wrapper in `web`. CI fails when the generated file differs from the committed file.

**Alternative considered**: hand-written types. Rejected: drift from the contract.

## 12. Internal API policy

**Decision** (decisions items 3, 6):

- One service now. No internal network API exists.
- The Go worker runs the Rust scoring binary as a local process. River and PostgreSQL coordinate ingestion and stored scores.
- If a synchronous internal call becomes necessary, use ConnectRPC with protobuf, defined under `contracts/`. Not now.
- Metrics (`:9090/metrics`) and probes are not public API. The ingress does not route port 9090.
