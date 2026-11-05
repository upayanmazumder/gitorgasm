# ADR 0007: Tenant authorization from GET /user/installations

- Status: Accepted
- Date: 2026-10-26

## Context

The tenant is a GitHub App installation (one organization or account). The legacy app let any user link any installation (`docs/architecture/legacy-audit.md`). Authorization must come from GitHub, not from user input.

## Decision

- Write the grant table `user_installations` only from `GET /user/installations`, called with the user's own token. Replace the full set for that user on each call. The list is a candidate set: GitHub also returns installations where the user only collaborates on one repository ([GitHub REST: list app installations accessible to the user access token](https://docs.github.com/en/rest/apps/installations#list-app-installations-accessible-to-the-user-access-token)). Keep an organization installation only when `GET /user/memberships/orgs/{org}` returns `state: active` with role `admin` or `member`. Keep a user-account installation only when the account is the signed-in user. Drop everything else.
- Join every organization-scoped query to `user_installations`.
- Return 404, not 403, for organizations the user cannot access, so existence does not leak.
- Installation deletion revokes all grants for that installation.
- Re-verify every session's grants with its own user token when they are older than 15 minutes (`sessions.grants_verified_at`), before serving the request. If GitHub cannot answer, refuse the request (503) and keep the stored grants. Webhooks are not used for revocation: they are unordered hints, and the `membership` event concerns teams.
- **Planned**: a server-side check of `GET /user/memberships/orgs/{org}` (role `admin`) before any mutation. Administration already does this live (`internal/administration`).

Implementation: **Verified** `core/internal/httpapi/auth/oauth.go`, `internal/store/tenancy`, `internal/httpapi/organization`.

## Consequences

- The legacy defect is closed by design.
- A removed organization member loses access on the first request made more than 15 minutes after the last confirmation (risk R10), instead of up to the session lifetime.
- Members see the whole installation, even when their repository access is narrower. Per-user repository filtering is not built.
- Each re-verification costs one paginated `GET /user/installations` plus one membership call per organization, per session, per 15 minutes of activity.
- The App needs organization permission "Members" read for the membership call (**Verified**: GitHub permissions page). **Assumption**: without it GitHub answers 403 or 404, so no organization grant is issued. Confirm the permission on the live App before deploying.
- With expiring user tokens, a session cannot be re-verified after the token expires (refresh is not implemented) and the user signs in again.

## Alternatives

- Trust an installation id from the setup URL callback: rejected. The `installation_id` query parameter can be forged.
- Check access per request against GitHub: rejected. Adds latency and rate-limit cost to every request.
