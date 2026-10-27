# ADR 0012: Argo CD deploys from this repository

- Status: Accepted
- Date: 2026-10-27
- Supersedes: [ADR 0010](0010-kustomize-argocd-cluster-repo.md)

## Context

ADR 0010 kept a production copy of the manifests and the SOPS Secrets in `upayanmazumder/cluster`. Each deploy needed a manual copy and a second pull request in the cluster repository. The two copies can drift. Each new image also needed a pull request to pin the `sha-<commit>` tag.

## Decision

- The Argo CD Application `gitorgasm` stays registered in the cluster repository (`k8s/argocd/applications/apps/gitorgasm.yaml`). Its source is `https://github.com/upayanmazumder/gitorgasm`, `targetRevision: main`, path `deploy/argocd`. Sync is automatic with prune and selfHeal.
- `deploy/argocd/kustomization.yaml` uses `../kustomize/overlays/production` and the ksops generator `secret-generator.yaml`. The generator reads `secrets.sops.yaml` (Secrets `gitorgasm-core` and `gitorgasm-postgres`). The root `.sops.yaml` encrypts this file with SOPS and age to the two cluster recipients. Only the Argo CD repo-server decrypts it.
- `deploy/argocd/check-encrypted.sh` fails if a `data` or `stringData` value is not `ENC[...]`. `make manifests` and the Manifests CI workflow run it.
- CI pushes `ghcr.io/upayanmazumder/gitorgasm/core:latest` and `sha-<commit>` on every push to `main`. Argo CD Image Updater tracks the digest of `latest` with write-back method `argocd`. The updater sets the kustomize image override on the Application. It does not make a git commit.
- A new AppProject `gitorgasm` in the cluster repository allows only this repository as a source and only namespace `gitorgasm` as a destination. Cluster-scoped kinds are limited to Namespace and PersistentVolume.
- The cluster repository keeps the Application, the AppProject, the ImageUpdater entry, node storage (`/srv/data/gitorgasm-postgres`), backup targets, and DNS.

Flow: merge to `main` → CI builds the image → the updater rolls api and worker in approximately 2 minutes. Argo CD syncs manifest changes under `deploy/` after merge (polling, approximately 3 minutes). Never apply manifests by hand.

## Consequences

- One copy of the manifests. No manual copy step per deploy.
- Review moves to pull requests in this repository. `main` is protected and requires the `CI` check. A merge to `main` is a production deploy.
- The updater follows the `latest` digest. The deployed image is not pinned in git. Rollback is a revert: revert the manifest commit on `main`, or revert the code commit so that CI pushes a new `latest`.
- Secret values are in this repository, encrypted. A plaintext commit is a risk. `check-encrypted.sh` blocks it in CI.

## Alternatives

- Keep the copies in the cluster repository (ADR 0010): rejected. Two copies drift, and each deploy needs a manual step.
- Image updater git write-back of the `sha-<commit>` tag: rejected. `main` is protected, so the updater cannot push to it.
- Multi-source Application with Secrets in the cluster repository: rejected. Image updater write-back on multi-source Applications is less proven, and ownership of the deploy splits across two repositories.
