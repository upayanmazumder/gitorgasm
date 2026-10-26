# ADR 0002: Modular monolith Go core with api, worker, and migrate roles

- Status: Accepted
- Date: 2026-10-26

## Context

The workload is GitHub orchestration: OAuth login, repository and membership sync, webhooks, and later bulk operations. The cluster has about 1.5Gi free memory requests on one node (**Verified** `docs/architecture/cluster-audit.md` §2). Each extra service costs memory, CI, and deployment work. The legacy app mixed UI and server logic in one Next.js app (`docs/architecture/legacy-audit.md`).

## Decision

Build one Go module `services/core` with internal packages `config`, `db`, `github`, `store`, `jobs`, `httpapi`, `obs`, `secure`. Build one image. The first argument selects the role (**Verified** `services/core/cmd/gitorgasm/main.go`):

- `api`: HTTP API on `:8080`, metrics on `:9090`.
- `worker`: River job processing, metrics and probes on `:9090`.
- `migrate`: applies goose and River migrations, then exits.

Deploy `api` and `worker` as two Deployments. Run `migrate` as an initContainer of `api` (**Verified** `deploy/kustomize/base/api.yaml`, details in `docs/architecture/deployment-topology.md` §8). All SQL lives in `store`. Packages talk through Go interfaces, not the network.

## Consequences

- One version runs in all roles. Schema and code cannot drift between api and worker.
- API latency is isolated from job load because roles run in separate pods.
- Module boundaries depend on discipline (internal packages), not on the network.
- A future split (for example the Rust scoring service, ADR 0003) communicates through PostgreSQL only.

## Alternatives

- Microservices per domain (auth, sync, webhooks): rejected. More memory, more deployments, distributed transactions for no current need.
- Single process running api and worker together: rejected. A job burst would affect API latency and a crash would stop both.
- Separate images per role: rejected. Version drift risk. `cluster-audit.md` §4 proposed two images. This ADR overrides that proposal. **Verified**: CI and `deploy/kustomize/base/kustomization.yaml` use the single image `ghcr.io/upayanmazumder/gitorgasm/core`.
