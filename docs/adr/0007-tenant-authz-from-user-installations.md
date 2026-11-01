# ADR 0007: Tenant authorization from GET /user/installations

- Status: Accepted
- Date: 2026-10-26

## Context

The tenant is a GitHub App installation (one organization or account). The legacy app let any user link any installation (`docs/architecture/legacy-audit.md`). Authorization must come from GitHub, not from user input.

## Decision

- Write the grant table `user_installations` only from `GET /user/installations`, called with the user's own token at login. Replace the full set for that user on each call. GitHub returns only installations that the user can access ([GitHub REST: list app installations accessible to the user access token](https://docs.github.com/en/rest/apps/installations#list-app-installations-accessible-to-the-user-access-token)).
- Join every organization-scoped query to `user_installations`.
- Return 404, not 403, for organizations the user cannot access, so existence does not leak.
- Installation deletion revokes all grants for that installation.
- **Planned** Stage 2: periodic re-verification of grants, and a server-side check of `GET /user/memberships/orgs/{org}` (role `admin`) before any mutation.

Implementation: **Verified** `core/internal/httpapi/auth.go`, `internal/store/tenancy`, `internal/httpapi/orgs.go`.

## Consequences

- The legacy defect is closed by design.
- Grants can be stale until the next login or session expiry (risk R10). No mutations exist yet, so the exposure is read access to repository metadata.
- Each login costs one paginated GitHub request.

## Alternatives

- Trust an installation id from the setup URL callback: rejected. The `installation_id` query parameter can be forged.
- Check access per request against GitHub: rejected. Adds latency and rate-limit cost to every request.
