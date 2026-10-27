# Kubernetes manifests

- `base/`: environment-neutral resources. Namespace `gitorgasm` (Pod Security `restricted` enforced), ConfigMap `gitorgasm-core` (generated from `config.env`), PostgreSQL StatefulSet, `api` Deployment (runs `migrate` as an init container), `worker` Deployment, NetworkPolicies (default deny).
- `overlays/production/`: the single-node k3s cluster. It pins pods to node `vps`, binds PostgreSQL to a static Retain PV on `/srv/data/gitorgasm-postgres`, adds the Ingress for `api-gitorgasm.upayan.dev`, and merges the frontend origin from its own `config.env`.

Validate with `make manifests`. CI renders the production overlay and schema-validates it in `.github/workflows/manifests.yml`.

## Configuration

Non-secret settings are in `base/config.env` and `overlays/production/config.env`. The variable names are the names that `core/internal/config/config.go` reads. Kustomize adds a content hash to the ConfigMap name, so a changed value rolls the pods that use it.

The worker overrides `DB_MAX_CONNS` in `base/worker.yaml`. The value must be at least 2 × `WORKER_CONCURRENCY` + 5, or the worker does not start.

The API init container runs goose before River. Existing production databases already have goose version 1 (`core/internal/db/migrations/00001_core.sql`); new releases apply `00002_schema_hardening.sql` and `00003_concurrent_indexes.sql` forward, then later versions in order. Never replace or renumber version 1. If a migration fails, the new API pod does not start and the old replica continues serving; do not run a down migration in production.

## Secrets

`deploy/argocd/secrets.sops.yaml` holds two Secrets, encrypted with SOPS and age to the two cluster recipients in the root `.sops.yaml`. The ksops generator in `deploy/argocd` adds them. Only the Argo CD repo-server decrypts them. `deploy/argocd/check-encrypted.sh` (run by `make manifests` and CI) fails if a value is not `ENC[...]`.

| Secret               | Keys                                                                                                                                                                  |
| -------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `gitorgasm-core`     | `DATABASE_URL`, `TOKEN_ENCRYPTION_KEY`, `GITHUB_APP_ID`, `GITHUB_APP_SLUG`, `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, `GITHUB_PRIVATE_KEY`, `GITHUB_WEBHOOK_SECRET` |
| `gitorgasm-postgres` | `POSTGRES_PASSWORD`                                                                                                                                                   |

`DATABASE_URL` is `postgres://gitorgasm:<password>@postgres.gitorgasm.svc:5432/gitorgasm?sslmode=disable`.

## Image

`base/kustomization.yaml` sets the tag `latest`. `.github/workflows/image.yml` pushes `ghcr.io/upayanmazumder/gitorgasm/core:latest` and `sha-<full commit SHA>` on every push to `main`. Argo CD Image Updater tracks the digest of `latest` with write-back method `argocd`. It sets the image override on the Application and makes no git commit. To roll back an image, revert the code commit on `main`. CI then pushes a new `latest`.

## Network

The NetworkPolicies allow only these paths. Every other path is denied.

- DNS to `kube-system/kube-dns` (UDP and TCP 53) from every pod.
- Traefik (`kube-system`, label `app.kubernetes.io/name=traefik`) to `api` on TCP 8080.
- Namespace `monitoring` to `api` and `worker` on TCP 9090.
- `api` and `worker` to `postgres` on TCP 5432, and to public IPv4 addresses on TCP 443 (GitHub).

An OTLP collector inside the cluster is not reachable. Add an egress rule before you set `OTEL_EXPORTER_OTLP_ENDPOINT`.

## Deployment

Argo CD Application `gitorgasm` (registered in `upayanmazumder/cluster`) reads `deploy/argocd` from `main` of this repository. Sync is automatic with prune and selfHeal. The first deploy was `upayanmazumder/cluster#77`. Status and open items are in `docs/architecture/implementation-roadmap.md` (Stage 1.5). The decision is ADR 0012.

To change production:

1. Change the manifests here and run `make manifests`.
2. Open a pull request in this repository. Wait for the `CI` check and review.
3. Merge to `main`. Argo CD syncs manifest changes in approximately 3 minutes. A new image rolls api and worker in approximately 2 minutes.
4. To roll back, revert the commit on `main`.
5. Never apply manifests to the live cluster by hand.

To edit a secret, run `sops deploy/argocd/secrets.sops.yaml`. You need one of the age keys. Commit only the encrypted file.

The first deploy needed these steps. A rebuild of the node needs them again:

1. Create `/srv/data/gitorgasm-postgres` on the node, owned by UID/GID 70.
2. Add the Argo CD Application, the `gitorgasm` AppProject, and the ImageUpdater entry in the cluster repository.
3. Make the GHCR package readable by the cluster. The package is public.
4. Add a restic backup target for a `pg_dump` of the database.
