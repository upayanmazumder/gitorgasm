# Architecture decision records

Format: ADR 0001. Status date: 2026-10-26.

| ADR                                                  | Title                                                                               | Status             |
| ---------------------------------------------------- | ----------------------------------------------------------------------------------- | ------------------ |
| [0001](0001-record-architecture-decisions.md)        | Record architecture decisions                                                       | Accepted           |
| [0002](0002-modular-monolith-go-core.md)             | Modular monolith Go core with api, worker, and migrate roles                        | Accepted           |
| [0003](0003-go-core-rust-for-scoring.md)             | Go for the core, Rust reserved for the scoring engine                               | Accepted           |
| [0004](0004-single-postgresql-store.md)              | Single PostgreSQL instance as the only stateful store                               | Accepted           |
| [0005](0005-river-durable-jobs.md)                   | River Postgres-backed durable jobs with transactional enqueue                       | Accepted           |
| [0006](0006-github-app-user-auth-sessions.md)        | GitHub App user-to-server auth with opaque server sessions and sealed tokens        | Accepted           |
| [0007](0007-tenant-authz-from-user-installations.md) | Tenant authorization from GET /user/installations                                   | Accepted           |
| [0008](0008-rest-openapi-same-origin-proxy.md)       | REST and OpenAPI public API with a same-origin Vercel proxy                         | Accepted           |
| [0009](0009-webhooks-as-hints-reconciliation.md)     | Webhooks as hints plus periodic reconciliation                                      | Accepted           |
| [0010](0010-kustomize-argocd-cluster-repo.md)        | Kustomize manifests deployed by the existing Argo CD through the cluster repository | Superseded by 0012 |
| [0011](0011-frontend-vercel-hobby.md)                | Frontend on Vercel Hobby from the template                                          | Accepted           |
| [0012](0012-argocd-deploys-from-this-repository.md)  | Argo CD deploys from this repository                                                | Accepted           |
