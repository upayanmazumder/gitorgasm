# AGENTS.md: Gitorgasm

Guide for coding agents and new engineers. Read [CONTRIBUTING.md](CONTRIBUTING.md) first.

## Layout

| Path                            | What it is                                                                                                                                                                     |
| ------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `core`                          | Go module. One binary, three roles: `gitorgasm api`, `gitorgasm worker`, `gitorgasm migrate`.                                                                                  |
| `core/internal/config`          | Environment loading and validation. All variables are documented in `.env.example`.                                                                                            |
| `core/internal/db`              | pgx pool, goose migrations (`migrations/`), River migrations.                                                                                                                  |
| `core/internal/github`          | Narrow GitHub REST client. Every endpoint was checked against the pinned REST description.                                                                                     |
| `core/internal/store`           | All SQL.                                                                                                                                                                       |
| `core/internal/jobs`            | River jobs: `sync_installation`, `process_webhook`, `reconcile_installations`.                                                                                                 |
| `core/internal/httpapi`         | HTTP handlers and middleware. The contract is `contracts/openapi/v1.yaml`.                                                                                                     |
| `core/internal/testdb`          | Gives each test an isolated, migrated database.                                                                                                                                |
| `web`                           | Next.js frontend (Vercel). It has its own `AGENTS.md`.                                                                                                                         |
| `scoring`                       | Rust library crate: contributor, repository, and team scoring. Golden fixtures in `scoring/tests/golden` come from the legacy engine (`generate.ts`). Never edit them by hand. |
| `deploy/kustomize`              | Kubernetes base and production overlay.                                                                                                                                        |
| `docs/architecture`, `docs/adr` | Architecture, audits, decisions.                                                                                                                                               |
| `tests/performance/lang-bench`  | Go vs Rust benchmark harness and recorded results.                                                                                                                             |

## Commands

```bash
make up        # local stack
make test      # all tests (needs Docker)
make lint      # all linters
make verify    # what CI runs
make manifests # render and validate Kubernetes manifests
```

## Rules

- GitHub behavior must be verified against docs.github.com or the pinned REST description (`github/rest-api-description` v2.1.0). Do not invent endpoints, fields, permissions, or webhook events. If you cannot verify a fact, stop and report it.
- Every org-scoped query must join `user_installations`. Return 404, not 403, for organizations the caller cannot access.
- Jobs that follow a data change use `InsertTx` in the same transaction.
- A sync pass may mark rows deleted only after every page succeeded.
- Never log or commit tokens, cookies, webhook payloads, or data exports.
- Change `contracts/openapi/v1.yaml` together with handlers. Tests enforce the contract.
- Do not add a new service, database, or broker without an ADR.
- Production deployment goes through the cluster repository (`upayanmazumder/cluster`) with a pull request. Do not apply manifests to the live cluster by hand.
