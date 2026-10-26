# Kubernetes manifests

- `base/`: environment-neutral resources. Namespace `gitorgasm` (Pod Security `restricted` enforced), ConfigMap, PostgreSQL StatefulSet, `api` Deployment (runs `migrate` as an init container), `worker` Deployment, NetworkPolicies (default deny).
- `overlays/production/`: the single-node k3s cluster. It pins pods to node `vps`, binds PostgreSQL to a static Retain PV on `/srv/data/gitorgasm-postgres`, adds the Ingress for `api-gitorgasm.upayan.dev`, and sets the frontend origin.

Validate with `make manifests`.

## Secrets (not in this repository)

The cluster repository provides two Secrets, encrypted with SOPS and age:

| Secret               | Keys                                                                                                                                                                  |
| -------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `gitorgasm-core`     | `DATABASE_URL`, `TOKEN_ENCRYPTION_KEY`, `GITHUB_APP_ID`, `GITHUB_APP_SLUG`, `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, `GITHUB_PRIVATE_KEY`, `GITHUB_WEBHOOK_SECRET` |
| `gitorgasm-postgres` | `POSTGRES_PASSWORD`                                                                                                                                                   |

`DATABASE_URL` is `postgres://gitorgasm:<password>@postgres.gitorgasm.svc:5432/gitorgasm?sslmode=disable`.

## Deployment (not done yet)

Deployment follows the cluster repository conventions (`docs/architecture/deployment-topology.md`):

1. Create `/srv/data/gitorgasm-postgres` on the node, owned by UID/GID 70.
2. Copy the production overlay into `k8s/apps/gitorgasm/` in the cluster repository. Add the SOPS secrets, an Argo CD Application, and the namespace in the `apps` AppProject. Open a pull request.
3. Make the GHCR package readable by the cluster (public package or an image pull secret).
4. Create the DNS record `api-gitorgasm.upayan.dev` in Cloudflare. This step needs owner approval.
5. Add a restic backup target for a `pg_dump` of the database.
