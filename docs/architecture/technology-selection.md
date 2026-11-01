# Technology selection

This document records each technology option, the verdict, and the reason. Each reason links to a cluster capacity fact or to the workload model.

## 1. Constraints that drive every choice

| Constraint                | Value                                                                                                                                                             | Source                                                      |
| ------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------- |
| Nodes                     | One k3s node `vps`, 4 vCPU, 7746Mi allocatable                                                                                                                    | **Verified**: [cluster-audit.md](./cluster-audit.md) §1–2   |
| Memory request headroom   | ~1,508Mi free. ~870Mi if all five KEDA scale-to-zero apps wake. Memory is the binding constraint, not CPU.                                                        | **Verified**: cluster-audit.md §2                           |
| CPU request headroom      | ~1,120m                                                                                                                                                           | **Verified**: cluster-audit.md §2                           |
| Present platform services | Argo CD (app-of-apps), SOPS + age via ksops, Traefik behind Cloudflare, Prometheus + Grafana, Loki + Promtail, KEDA, argocd-image-updater, restic → Cloudflare R2 | **Verified**: cluster-audit.md                              |
| Absent platform services  | CloudNativePG, NATS, Valkey/Redis (shared), Tempo                                                                                                                 | **Verified**: cluster-audit.md §3 (`kubectl get crd`, pods) |
| Storage                   | `local-path` (default, `Delete`), `hcloud-volumes`. Static `hostPath` `Retain` PVs used by existing apps.                                                         | **Verified**: cluster-audit.md §6                           |
| Budget                    | No paid resources beyond the VM. Frontend on Vercel Hobby.                                                                                                        | Integrator constraint                                       |
| Workload                  | I/O- and rate-limit-bound at every scale. Small data below Stage 5.                                                                                               | [workload-model.md](./workload-model.md) §5                 |

**Decision**: every new stateful service must justify its memory request against the ~1.5Gi headroom. The planned Gitorgasm total is 512Mi of requests (api 128Mi, worker 128Mi, Postgres 256Mi; D15).

## 2. Decisions

### 2.1 PostgreSQL deployment

| Option                                                                                                                      | Verdict      | Reason                                                                                                                                                                                                         |
| --------------------------------------------------------------------------------------------------------------------------- | ------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Plain StatefulSet, `postgres:18`, static `hostPath` `Retain` PV at `/srv/data/gitorgasm-postgres`, restic class-A `pg_dump` | **Selected** | Matches the existing `vcap-v1` pattern (cluster-audit.md §6). No operator memory. Backups use the existing restic pipeline to R2.                                                                              |
| CloudNativePG operator + `Cluster`                                                                                          | Rejected now | CNPG is not installed (**Verified**). The operator adds a controller pod and CRDs. Its main value is replicas and failover, which need a second node. On one node, HA is not real. Revisit with a second node. |
| Managed Postgres (Neon, Supabase, RDS)                                                                                      | Rejected     | Paid at useful size, or adds a cross-internet hop. Legacy Supabase shows the risk: staging/prod projects are gone (NXDOMAIN).                                                                                  |
| SQLite                                                                                                                      | Rejected     | River needs Postgres. Multiple api/worker processes need a network database.                                                                                                                                   |

**Verified**: local dev uses `postgres:18-alpine` (`compose.yaml`). Migrations: goose plus River migrations run by the `migrate` role (`core/internal/db/db.go`, `core/cmd/gitorgasm/main.go`).

### 2.2 Cache

| Option           | Verdict         | Reason                                                                                                                                                                                                                                                      |
| ---------------- | --------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| No cache service | **Selected**    | Read paths query Postgres only, no GitHub call (**Verified**: `core/internal/httpapi/orgs.go`). Sessions, locks (advisory locks in `jobs/internal/execution`), and the queue live in Postgres. Legacy Redis was a response cache only and caused SEC-1 (cross-tenant leak). |
| Valkey           | Rejected now    | Adds ~50–100Mi requests (**Assumption**) for no current need. Revisit only if measured p95 read latency fails the target in [requirements.md](./requirements.md) §6 after indexing.                                                                         |
| Dragonfly        | Rejected        | Same as Valkey, with a larger default memory footprint (**Assumption**).                                                                                                                                                                                    |
| In-process cache | Allowed, narrow | Installation tokens are cached in memory today (**Verified**: `github/tokens.go`). Any other in-process cache must be tenant-keyed.                                                                                                                         |

### 2.3 Messaging and workflow

| Option                        | Verdict      | Reason                                                                                                                                                                                                                                    |
| ----------------------------- | ------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| River (Postgres-backed queue) | **Selected** | Transactional enqueue with the data change (**Verified**: `InsertTx` in `httpapi/orgs.go`, `httpapi/webhooks.go`, `httpapi/auth.go`). Unique jobs, snooze, periodic jobs, retries. No new stateful service. v0.49 (**Verified**: `core/go.mod`). |
| NATS JetStream                | Rejected now | Not installed. Adds a stateful service and a second source of truth. Fan-out to many consumers is not needed with one service. Revisit if several services need the same event stream.                                                    |
| Temporal                      | Rejected     | Needs its own server, history store, and UI. Several hundred Mi of memory (**Assumption**). River jobs with checkpoints in Postgres cover bulk operations (Stage 4).                                                                      |
| Kafka / Redpanda              | Rejected     | Far beyond the event volume in workload-model.md §2.3.                                                                                                                                                                                    |

### 2.4 Analytics store

| Option                                                                            | Verdict                          | Reason                                                                                                                                                |
| --------------------------------------------------------------------------------- | -------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| Postgres rollup tables, range partitioning for `signals`, materialized aggregates | **Selected** (Planned, Stage 5+) | Legacy data is tiny (50 `signals` rows, database-audit.md). 10–100 M rows/year at the largest scale (workload-model.md §4) fits partitioned Postgres. |
| ClickHouse                                                                        | Rejected now                     | Large memory footprint (**Assumption**: ≥1Gi for comfort) would consume most headroom. Revisit when a rollup query misses its target on real data.    |

### 2.5 Search

| Option                                 | Verdict                | Reason                                                       |
| -------------------------------------- | ---------------------- | ------------------------------------------------------------ |
| `pg_trgm` index on names and logins    | **Selected** (Planned) | Fuzzy match for repository and member names. No new service. |
| Postgres full-text search (`tsvector`) | Optional, later        | Only if descriptions or text bodies become searchable.       |
| Meilisearch                            | Rejected               | Separate stateful service and index sync for small data.     |
| OpenSearch                             | Rejected               | JVM memory needs exceed the headroom.                        |

### 2.6 Object storage

| Option                                           | Verdict        | Reason                                                                                             |
| ------------------------------------------------ | -------------- | -------------------------------------------------------------------------------------------------- |
| Cloudflare R2 for backups only                   | **Selected**   | Already used by the cluster restic pipeline (**Verified**: cluster-audit.md §6).                   |
| R2 or MinIO for app files (CSV uploads, exports) | Not needed now | Mass-invite input is parsed and submitted as JSON (Planned Stage 4). Exports stream from Postgres. |
| MinIO in cluster                                 | Rejected       | Stateful, memory cost, no need.                                                                    |

### 2.7 Kubernetes packaging

| Option                                 | Verdict      | Reason                                                                                                                                                                    |
| -------------------------------------- | ------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Kustomize (base + overlays/production) | **Selected** | Matches the cluster repo conventions. Argo CD renders it natively. **Verified**: `deploy/kustomize` exists and passes kubeconform (CI job `manifests`, `make manifests`). |
| Helm chart                             | Rejected now | Templating adds indirection for one environment. Revisit if third parties install Gitorgasm.                                                                              |

### 2.8 GitOps controller

| Option             | Verdict      | Reason                                                                                                                                                                                                          |
| ------------------ | ------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Argo CD (existing) | **Selected** | Already runs the app-of-apps from `upayanmazumder/cluster` with SOPS/ksops and image updater (**Verified**: cluster-audit.md). The Application reads `deploy/argocd` from `main` of this repository (ADR 0012). |
| Flux               | Rejected     | A second GitOps controller duplicates Argo CD and its memory.                                                                                                                                                   |

### 2.9 API style

| Option                                   | Verdict                     | Reason                                                                                                                                                                          |
| ---------------------------------------- | --------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| REST + JSON, OpenAPI 3.0 contract, `/v1` | **Selected**                | Browser-friendly through a same-origin rewrite. Contract at `contracts/openapi/v1.yaml`, validated against real responses in tests (D6). Simple to cache at the edge if needed. |
| GraphQL                                  | Rejected for the public API | Legacy GraphQL proxy had auth and allowlist defects (feature-inventory §2.3). Cost control per query is harder. Gitorgasm still consumes GitHub GraphQL for ingest (Stage 5).   |
| ConnectRPC / gRPC                        | Not now                     | No internal service-to-service calls exist. If a synchronous internal RPC is ever needed, use ConnectRPC with protobuf (D6, Planned).                                           |
| SSE for progress                         | Planned (Stage 4)           | One-way progress fits SSE. WebSockets are not needed.                                                                                                                           |

### 2.10 Frontend

| Option                                                                                     | Verdict                | Reason                                                                                                                                                                                                 |
| ------------------------------------------------------------------------------------------ | ---------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Next.js + TypeScript strict + TanStack Query + shadcn/ui + Tailwind, pnpm, on Vercel Hobby | **Selected** (D7)      | Free hosting. Team knows the stack from legacy. Same-origin rewrite `/api/:path*` → `https://api-gitorgasm.upayan.dev/:path*` keeps the session cookie first-party. No pod memory used in the cluster. |
| Frontend in the cluster                                                                    | Rejected               | Uses memory headroom for static and SSR work Vercel provides free.                                                                                                                                     |
| SPA (Vite) on Cloudflare Pages                                                             | Alternative considered | Viable. Rejected to keep one stack with the template and the rewrite pattern.                                                                                                                          |

**Note**: Vercel Hobby is for non-commercial use (D7). A commercial use needs a different host.

### 2.11 Observability

| Option                                                                      | Verdict                             | Reason                                                                                                            |
| --------------------------------------------------------------------------- | ----------------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| Prometheus scrape via pod annotations, Loki via stdout JSON                 | **Selected**                        | Existing stack (**Verified**: cluster-audit.md). Code exposes `/metrics` on `:9090` (`core/internal/obs/obs.go`). |
| OTel traces with OTLP/HTTP export when `OTEL_EXPORTER_OTLP_ENDPOINT` is set | **Selected**, export off by default | No Tempo in the cluster. Instrumentation exists (`obs.InitTracing`). A trace backend is Planned (Stage 6).        |

## 3. Memory budget summary

| Component                    | Request                                                       | Limit   | Source         |
| ---------------------------- | ------------------------------------------------------------- | ------- | -------------- |
| api                          | 128Mi                                                         | 256Mi   | D15            |
| worker                       | 128Mi                                                         | 256Mi   | D15            |
| Postgres                     | 256Mi                                                         | 512Mi   | D15            |
| Total                        | 512Mi                                                         | 1,024Mi | —              |
| Rejected options (not added) | Valkey, NATS, Temporal, ClickHouse, OpenSearch, CNPG operator | —       | sections above |

**Estimate**: 512Mi is about 34% of the ~1,508Mi request headroom, or about 59% of the ~870Mi worst case. Any additional stateful service must fit in what remains.
