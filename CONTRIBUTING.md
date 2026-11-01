# Contributing

## Workflow

1. Open an issue for any change larger than a small fix.
2. Create a branch from `main`. Use a short name such as `feat/team-sync` or `fix/cursor-decode`.
3. Keep each pull request to one concern.
4. Run `make verify` before you push. CI runs area-scoped checks through reusable workflows; its dependency audits and image build are separate jobs.
5. Open a pull request. The `main` ruleset requires a pull request, resolved review threads, and the single `CI` status check from `.github/workflows/ci.yml`. It blocks force pushes and branch deletion. The job `changes` detects the changed areas, and each stage runs only when its area changed. A skipped stage counts as success in `CI`, so path filtering never blocks the required check.
6. Request a review from the code owner for security-sensitive paths (`.github/CODEOWNERS`). Required approvals are 0 while the project has one maintainer. Raise the count when a second maintainer joins.

Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/): `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`.

### Formatters and git hooks

Run `pnpm install` at the repository root once. It installs husky hooks. The root `package.json` holds only git tooling. Each language keeps its formatter config next to its code:

| Code                              | Formatter                              | Config                                                   |
| --------------------------------- | -------------------------------------- | -------------------------------------------------------- |
| `web/`                            | Prettier and ESLint `--fix`            | `web/.prettierrc` (import sorting, Tailwind class order) |
| `core/`                           | `golangci-lint fmt` (gofmt, goimports) | `core/.golangci.yml`                                     |
| `tests/performance/lang-bench/go` | `golangci-lint fmt` (gofmt, goimports) | `tests/performance/lang-bench/go/.golangci.yml`          |
| Rust crates                       | `rustfmt`                              | `rustfmt.toml` in each crate                             |
| Everything else                   | none                                   | `.editorconfig` (charset, line endings, indentation)     |

- `pre-commit` runs lint-staged on staged files only, with the formatters above. It needs `go`, `rustfmt`, and `pnpm` on `PATH`. golangci-lint runs through `go run` at the pinned version.
- `commit-msg` runs commitlint (Conventional Commits).
- `pre-push` checks the committed paths being pushed. New branches compare with their merge base on `origin/main`. Deleted branches need no checks.
- Web changes run lint, type checks, formatting checks, and frontend tests. Markdown-only web changes skip these checks.
- Core and contract changes run `go vet`. Rust changes run formatting checks, clippy, and tests for the affected crate.
- Go benchmark changes run `go vet`. Deployment changes run `make manifests`. Workflow changes run actionlint.
- Hook changes run JavaScript syntax checks. Documentation-only pushes do not start code tests or Docker.
- Full integration tests, including PostgreSQL and Go race tests, remain in CI. Run `make verify` before opening a PR.

`make fmt` formats everything. `make lint` and each CI language job check the same rules.

## Local setup

See the quick start in [README.md](README.md). Tests need Docker: `make test` starts PostgreSQL on `127.0.0.1:55432`. Each Go test creates and drops its own database.

## Standards

### Go (`core`)

- All SQL lives in `internal/store`. Handlers and workers never build SQL.
- Every org-scoped query joins `user_installations`. Inaccessible organizations return 404.
- A job that follows a data change is inserted with `InsertTx` in the same transaction.
- GitHub calls go through `internal/github`. Verify each new endpoint against the pinned REST description (`github/rest-api-description` v2.1.0) or docs.github.com before you use it. Record the operationId in the doc comment.
- Return classified errors. Workers map them with `retryDecision`.
- Never log tokens, cookies, webhook payloads, or private repository content.
- Lint: `golangci-lint` with `core/.golangci.yml`.

### API contract

- `contracts/openapi/v1.yaml` is the source of truth. Change it in the same pull request as the handler.
- Tests validate real responses against the contract. A response that does not match fails the build.
- Breaking changes need a new version path.

### Database migrations

- Add a new goose-format file in `core/internal/db/migrations` (`NNNNN_name.sql`, next sequential version). `00001_core.sql` is the immutable deployed baseline; `00002_schema_hardening.sql` adds storage constraints, and `00003_concurrent_indexes.sql` replaces the login index and adds a retention index. Never edit or renumber a deployed migration.
- The `migrate` role runs goose then River migrations before a new API pod serves traffic. New migrations must work with the previous release: expand first, contract in a later release. Test both an empty database and an upgrade from the deployed schema; a fresh-database test alone cannot detect a rewritten migration.

### Frontend (`web`)

- Strict TypeScript, pnpm only. See `web/AGENTS.md`.

### Tests

- Test behavior that a user or operator can observe: tenant isolation, pagination limits, retry decisions, idempotency.
- Do not write tests that only repeat wiring or default values.

## Architecture decisions

Record significant decisions as ADRs in `docs/adr` (copy the format of an existing ADR). Update the index.
