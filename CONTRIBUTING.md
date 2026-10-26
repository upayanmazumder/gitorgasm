# Contributing

## Workflow

1. Open an issue for any change larger than a small fix.
2. Create a branch from `main`. Use a short name such as `feat/team-sync` or `fix/cursor-decode`.
3. Keep each pull request to one concern.
4. Run `make verify` before you push. CI runs the same checks.
5. Get one approving review. The code owner reviews changes to security-sensitive paths.

Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/): `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`.

## Local setup

See the quick start in [README.md](README.md). Tests need Docker: `make test` starts PostgreSQL on `127.0.0.1:55432`. Each Go test creates and drops its own database.

## Standards

### Go (`services/core`)

- All SQL lives in `internal/store`. Handlers and workers never build SQL.
- Every org-scoped query joins `user_installations`. Inaccessible organizations return 404.
- A job that follows a data change is inserted with `InsertTx` in the same transaction.
- GitHub calls go through `internal/github`. Verify each new endpoint against the pinned REST description (`github/rest-api-description` v2.1.0) or docs.github.com before you use it. Record the operationId in the doc comment.
- Return classified errors. Workers map them with `retryDecision`.
- Never log tokens, cookies, webhook payloads, or private repository content.
- Lint: `golangci-lint` with `services/core/.golangci.yml`.

### API contract

- `contracts/openapi/v1.yaml` is the source of truth. Change it in the same pull request as the handler.
- Tests validate real responses against the contract. A response that does not match fails the build.
- Breaking changes need a new version path.

### Database migrations

- Add a new file in `services/core/internal/db/migrations` (`NNNNN_name.sql`, goose format). Never edit a merged migration.
- Migrations run before the new API version starts. They must work with the previous release: expand first, contract in a later release.

### Frontend (`apps/web`)

- Strict TypeScript, pnpm only. See `apps/web/AGENTS.md`.

### Tests

- Test behavior that a user or operator can observe: tenant isolation, pagination limits, retry decisions, idempotency.
- Do not write tests that only repeat wiring or default values.

## Architecture decisions

Record significant decisions as ADRs in `docs/adr` (copy the format of an existing ADR). Update the index.
