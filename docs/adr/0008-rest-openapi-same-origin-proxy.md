# ADR 0008: REST and OpenAPI public API with a same-origin Vercel proxy

- Status: Accepted
- Date: 2026-10-26

## Context

The frontend (ADR 0011) and possible future clients need a stable API. Session cookies must be first-party. The team must not maintain a CORS policy.

## Decision

- REST with JSON under the versioned path `/v1`. The contract is OpenAPI 3.0 at `contracts/openapi/v1.yaml`. Tests validate real responses against it (kin-openapi).
- Error body `{"error":{"code","message","request_id"}}` (**Verified** `core/internal/httpapi/server.go` `writeError`).
- Keyset pagination with an opaque cursor (**Verified** `internal/httpapi/orgs.go`).
- Idempotency: sync requests are deduplicated by River unique jobs. Webhooks by `X-GitHub-Delivery`. **Planned** Stage 4: `Idempotency-Key` header for mutations. SSE for operation progress.
- The Vercel app rewrites `/api/:path*` to `https://api-gitorgasm.upayan.dev/:path*` (**Planned** config). The browser sees one origin, so the session cookie is first-party and no CORS is needed.
- No internal network API now (one service). If a synchronous internal RPC is ever needed, use ConnectRPC with protobuf through a new ADR.

## Consequences

- Clients and server share one contract. Breaking changes need `/v2`.
- The proxy adds one network hop through Vercel.
- Cookie scope is the frontend origin. The API host itself never sets cross-site cookies.

## Alternatives

- GraphQL: rejected. Adds a schema layer and query-cost control without a client need.
- tRPC: rejected. Couples the API to TypeScript and to one client.
- Direct browser calls with CORS and `SameSite=None` cookies: rejected. Third-party cookie restrictions and more CSRF surface.
