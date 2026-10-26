# ADR 0010: Kustomize manifests deployed by the existing Argo CD through the cluster repository

- Status: Accepted
- Date: 2026-10-26

## Context

The cluster uses Argo CD app-of-apps from `upayanmazumder/cluster`, with SOPS + age via ksops, and argocd-image-updater with digest write-back (**Verified** `docs/architecture/cluster-audit.md` §1, §3). The cluster `main` branch is protected (PR + seven CI checks).

## Decision

- Keep Kubernetes manifests in this repository at `deploy/kustomize` (base + `overlays/production`). CI job `manifests` renders them with kustomize and validates them with `kubeconform -strict` (`.github/workflows/ci.yml`, `make manifests`).
- Deploy through the cluster repository: `k8s/apps/gitorgasm`, an explicit Argo CD Application (not the single-image ApplicationSet), and a destination entry in the `apps` AppProject. Secrets `gitorgasm-core` and `gitorgasm-postgres` stay in the cluster repository as SOPS files.
- argocd-image-updater tracks `ghcr.io/upayanmazumder/gitorgasm/core` by digest and writes back to the cluster repository.
- Follow cluster conventions: node pin to `vps`, restricted security context, default-deny NetworkPolicies, Traefik Ingress without `tls:`, static Retain hostPath PV under `/srv/data`. One deliberate difference: the namespace enforces Pod Security `restricted` (`base/namespace.yaml`), stricter than the cluster component `namespace-baseline` (enforce `baseline`).

Status: manifests **Verified** in `deploy/kustomize` (Stage 1; the integrator validated 15 resources with kubeconform). Cluster PR **Planned** for Stage 1.5. Details in `docs/architecture/deployment-topology.md`.

## Consequences

- One review path for all cluster changes. The owner approves each deploy-affecting change.
- Manifests exist in two places (source here, copy or reference in the cluster repository). Keep them in sync in the deploy PR.
- No Helm chart to maintain.

## Alternatives

- Helm chart: rejected. Templating is not needed for one environment.
- Argo CD Application pointing at this repository: rejected for now. SOPS keys, AppProject, and image-updater write-back live in the cluster repository.
- Manual `kubectl apply`: rejected. Not GitOps, no drift detection.
