# Kubernetes manifests

- `base/`: environment-neutral resources. Namespace `gitorgasm` (Pod Security `restricted` enforced), ConfigMap `gitorgasm-core` (generated from `config.env`), PostgreSQL StatefulSet, `api` Deployment (runs `migrate` as an init container), `worker` Deployment, NetworkPolicies (default deny).
- `overlays/production/`: the single-node k3s cluster. It pins pods to node `vps`, binds PostgreSQL to a static Retain PV on `/srv/data/gitorgasm-postgres`, adds the Ingress for `api-gitorgasm.upayan.dev`, and merges the frontend origin from its own `config.env`.

Validate with `make manifests`. CI renders the production overlay and schema-validates it in `.github/workflows/manifests.yml`.

## Configuration

Non-secret settings are in `base/config.env` and `overlays/production/config.env`. The variable names are the names that `core/internal/config/config.go` reads. Kustomize adds a content hash to the ConfigMap name, so a changed value rolls the pods that use it.

The worker overrides `DB_MAX_CONNS` in `base/worker.yaml`. The value must be at least 2 × `WORKER_CONCURRENCY` + 5, or the worker does not start.

The API init container runs goose before River. Existing production databases already have goose version 1 (`core/internal/db/migrations/00001_core.sql`); new releases apply `00002_schema_hardening.sql` and `00003_concurrent_indexes.sql` forward, then later versions in order. Never replace or renumber version 1. If a migration fails, the new API pod does not start and the old replica continues serving; do not run a down migration in production.

## Secrets (not in this repository)

The cluster repository provides two Secrets, encrypted with SOPS and age:

| Secret               | Keys                                                                                                                                                                  |
| -------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `gitorgasm-core`     | `DATABASE_URL`, `TOKEN_ENCRYPTION_KEY`, `GITHUB_APP_ID`, `GITHUB_APP_SLUG`, `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, `GITHUB_PRIVATE_KEY`, `GITHUB_WEBHOOK_SECRET` |
| `gitorgasm-postgres` | `POSTGRES_PASSWORD`                                                                                                                                                   |

`DATABASE_URL` is `postgres://gitorgasm:<password>@postgres.gitorgasm.svc:5432/gitorgasm?sslmode=disable`.

## Image

`base/kustomization.yaml` sets the tag `latest` as a placeholder. `.github/workflows/image.yml` pushes `ghcr.io/upayanmazumder/gitorgasm/core:sha-<full commit SHA>` on every push to `main`. The cluster repository must pin `images[].newTag` to one of these tags. Argo CD then deploys a known build, and a rollback is a revert of one line.

## Network

The NetworkPolicies allow only these paths. Every other path is denied.

- DNS to `kube-system/kube-dns` (UDP and TCP 53) from every pod.
- Traefik (`kube-system`, label `app.kubernetes.io/name=traefik`) to `api` on TCP 8080.
- Namespace `monitoring` to `api` and `worker` on TCP 9090.
- `api` and `worker` to `postgres` on TCP 5432, and to public IPv4 addresses on TCP 443 (GitHub).

An OTLP collector inside the cluster is not reachable. Add an egress rule before you set `OTEL_EXPORTER_OTLP_ENDPOINT`.

## Deployment

Production runs from `k8s/apps/gitorgasm/` in the cluster repository (`upayanmazumder/cluster#77`). Argo CD syncs it. Status and open items are in `docs/architecture/implementation-roadmap.md` (Stage 1.5).

To change production:

1. Change the manifests here and run `make manifests`.
2. Copy the change into `k8s/apps/gitorgasm/` in the cluster repository. Pin the immutable image tag. Open a pull request and wait for its checks and review; Argo CD syncs after merge.
3. Do not apply manifests to the live cluster by hand or bypass the cluster PR with automatic image write-back.

The first deploy needed these steps. A rebuild of the node needs them again:

1. Create `/srv/data/gitorgasm-postgres` on the node, owned by UID/GID 70.
2. Add the SOPS Secrets, the Argo CD Application, and the namespace in the `apps` AppProject.
3. Make the GHCR package readable by the cluster. The package is public.
4. Add a restic backup target for a `pg_dump` of the database.
