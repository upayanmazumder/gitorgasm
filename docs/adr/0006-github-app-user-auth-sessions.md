# ADR 0006: GitHub App user-to-server auth with opaque server sessions and sealed tokens

- Status: Accepted
- Date: 2026-10-26

## Context

Users sign in with GitHub. The legacy app had authorization defects, including linking of any installation by any user (`docs/architecture/legacy-audit.md`). Browser code must never hold privileged tokens.

## Decision

- Use the GitHub App web flow for user access tokens with `state` and PKCE S256. Store the code verifier in a short-lived `__Host-` HttpOnly cookie. Exchange the code at `POST https://github.com/login/oauth/access_token` ([GitHub docs: generating a user access token for a GitHub App](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-user-access-token-for-a-github-app)).
- Issue an opaque 256-bit session id in cookie `__Host-gitorgasm_session` (HttpOnly, Secure, SameSite=Lax, Path=/). Store only the SHA-256 hash in table `sessions`. TTL 168 h (`SESSION_TTL`). Logout deletes the row.
- Seal the GitHub user token (and refresh token when present) with AES-256-GCM. Additional data = internal user id. Column `key_version` supports key rotation. Key from `TOKEN_ENCRYPTION_KEY`.
- The `github_app_authorization` webhook with action `revoked` deletes all sessions and all installation grants of that user in one transaction (`identity.RevokeGitHubUser`). The next login rebuilds the grants from `GET /user/installations`.
- CSRF: SameSite=Lax plus a required `Origin` equal to `APP_ORIGIN` on unsafe methods. The webhook route is exempt (HMAC).

Implementation: **Verified** `core/internal/httpapi/auth/oauth.go`, `internal/secure/secure.go`, `internal/httpapi/middleware` (`CheckOrigin`), `internal/store/identity` (`RevokeGitHubUser`), `internal/jobs/webhook` (webhook handler).

## Consequences

- A database leak does not expose usable session ids or tokens without the key.
- Sessions can be revoked server-side at once (unlike stateless JWTs).
- Each authenticated request reads the `sessions` table. Acceptable at current scale.
- Refresh-token use is not implemented yet (**Planned** Stage 2). Users re-login when a token expires.

## Alternatives

- OAuth App instead of GitHub App: rejected. GitHub App tokens are limited to the App's permissions and installations.
- Stateless JWT session cookie: rejected. No server-side revocation. Token must be inside or referenced anyway.
- Supabase Auth (legacy): rejected. Supabase is removed (ADR 0004).
