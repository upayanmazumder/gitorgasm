# Observability

Status date: 2026-10-26. Labels: **Verified** (with source), **Assumption**, **Decision**, **Alternative considered**, **Planned**.

Scope: the Go core in `services/core`. The cluster stack is plain Prometheus with pod-annotation scrape, Grafana, Loki, and Promtail. There is no Tempo and no Prometheus Operator (**Verified** `docs/architecture/cluster-audit.md` §3).

## 1. Logs

**Verified** (`internal/obs/obs.go` `NewLogger`): `log/slog` JSON handler to stdout, level `info`. Every record has these fields:

| Field | Source | Example |
| --- | --- | --- |
| `time`, `level`, `msg` | slog | `"msg":"request"` |
| `service` | constant | `gitorgasm` |
| `component` | process role (`os.Args[1]`) | `api`, `worker`, `migrate` |
| `app_version` | build flag `-X main.version`, attached by `obs.NewLogger(role).With("app_version", version)` (`cmd/gitorgasm/main.go`) | commit SHA, `dev` locally |
| `trace_id`, `span_id` | added by `traceHandler` when the context has a valid span | 32 / 16 hex chars |

Access log (**Verified** `internal/httpapi/server.go` `accessLog`): one `info` record `request` per request with `method`, `route` (the `net/http` pattern, for example `GET /v1/organizations/{login}/repositories`, or `unmatched`), `status`, `duration_ms`, `request_id`. `/healthz` and `/readyz` are not logged. Errors: `request failed` with `error`, `route`, `request_id` (`internalError`). Panics: `panic` with `value` and `stack` (`recoverer`).

### Request ids

**Verified** (`server.go` `requestID`): the API accepts an inbound `X-Request-Id` only if it matches `^[A-Za-z0-9._-]{8,64}$`. Otherwise it generates a 22-character random id. It echoes the id in the `X-Request-Id` response header. The error body carries it as `error.request_id`. A user can quote this id. An operator then finds the log line in Loki.

### Tenant-safe diagnostics

**Decision** (rules for all new code):

1. Never log tokens, cookies, session ids, `code`, `state`, PKCE verifiers, private keys, webhook secrets, or `Authorization` headers.
2. Never log webhook payloads or GitHub response bodies. Raw webhook bodies live only in `webhook_deliveries`.
3. Log ids, not names: installation id, internal user id, GitHub delivery id, River job id. **Verified**: the access log uses the route pattern, not the raw path, so organization logins in the URL path do not reach the `route` field. Current code does not log repository names.
4. Log the error kind and HTTP status of GitHub failures, not the GitHub message body.

**Assumption**: `store.SyncFinished` stores `err.Error()` in `sync_state` (`internal/jobs/jobs.go`). River also stores job errors in `river_job.errors`. Review the `github.APIError` string format in Stage 2 so it carries no token or private repository name.

## 2. Metrics

**Verified** (`internal/obs/obs.go` `MetricsServer`): `/metrics` is served on `METRICS_ADDR` (default `:9090`) by `promhttp.Handler()`. This port is separate from the API port and is not routed by the Ingress. The default Go and process collectors are registered.

| Metric | Type | Labels | Source |
| --- | --- | --- | --- |
| `gitorgasm_http_requests_total` | counter | `route`, `method`, `status` | `httpapi/server.go` |
| `gitorgasm_http_request_duration_seconds` | histogram, buckets 5 ms to 5 s | `route` | `httpapi/server.go` |
| `gitorgasm_github_requests_total` | counter | `operation`, `status` (`2xx`, `4xx`, `5xx`, `error`) | `github/client.go` |
| `gitorgasm_github_rate_limit_remaining` | gauge | `credential` (credential kind, not per installation) | `github/client.go` |
| `go_*`, `process_*` | default collectors | none | `client_golang` |

**Decision**: label values stay bounded. Use the route pattern, never the raw path. Never use installation ids, user ids, or logins as labels. Per-tenant detail goes to logs and to the `sync_state` table.

**Limitation (Verified)**: the rate-limit gauge holds the last observed value for a credential kind. With several installations it mixes values. It is a coarse signal. Per-installation budgets are visible in logs only.

**Planned** (Stage 2): River job metrics. River has no built-in Prometheus exporter. Subscribe to River events (`client.Subscribe` for completed, failed, snoozed, cancelled) and export:

- `gitorgasm_jobs_total{kind,outcome}` (counter)
- `gitorgasm_job_duration_seconds{kind}` (histogram)
- `gitorgasm_jobs_available{queue}` (gauge, from a periodic `SELECT count(*) FROM river_job WHERE state = 'available'`)
- `gitorgasm_sync_last_success_timestamp_seconds` (gauge, max over installations, no per-installation label)

### Scrape configuration

Pod annotations on `api` and `worker` (**Verified** in `deploy/kustomize/base/api.yaml` and `base/worker.yaml`. Cluster convention: `cluster-audit.md` §3, `k8s/monitoring/prometheus-configmap.yaml` lines 56-80):

```yaml
prometheus.io/scrape: "true"
prometheus.io/port: "9090"
prometheus.io/path: "/metrics"
```

**Verified** (`deploy/kustomize/base/networkpolicies.yaml`): the NetworkPolicy `metrics-ingress` allows TCP 9090 to `api` and `worker` from namespace `monitoring`. The Ingress does not route port 9090 or the health paths (`deployment-topology.md` §2).

## 3. Traces

**Verified** (`obs.InitTracing`, `server.go` `Handler`, `github/client.go`):

- W3C `traceparent` and `baggage` propagation (`propagation.TraceContext{}`, `propagation.Baggage{}`).
- Server spans by `otelhttp.NewHandler` for each request. Client spans by `otelhttp.NewTransport` for each GitHub call. The trace context therefore flows from the API request to the GitHub request.
- Service name `gitorgasm-api` or `gitorgasm-worker`.
- Export over OTLP/HTTP only when `OTEL_EXPORTER_OTLP_ENDPOINT` is set. Without it, spans are still created, so `trace_id` reaches logs.

**Limitation**: the trace does not cross the River queue. A sync job started by `POST /v1/organizations/{login}/sync` has a new trace. **Planned** (Stage 4): store `traceparent` in job metadata at insert and start the job span as a link.

**Decision**: no trace backend now. Tempo is not in the cluster and costs memory. **Planned** (Stage 6): evaluate Tempo in single-binary mode, or Grafana Cloud free tier, behind the existing env variable.

## 4. Health and readiness

| Endpoint | Process | Meaning | Source |
| --- | --- | --- | --- |
| `GET /healthz` | api `:8080`, worker `:9090` | process is up. No dependency checks. Use for liveness | **Verified** `server.go` `healthz`, `obs.go` |
| `GET /readyz` | api `:8080`, worker `:9090` | PostgreSQL answers `Ping` within 2 s. 503 otherwise | **Verified** `server.go` `readyz`, `obs.go` |

**Decision**: liveness never checks Postgres or GitHub. A database outage must not restart every pod in a loop. Readiness removes the api pod from the Service during a database outage. GitHub health is not part of readiness. GitHub failures are handled per job.

**Verified** (probe settings, `deploy/kustomize/base/api.yaml`, `base/worker.yaml`):

| Process | Startup | Liveness | Readiness |
| --- | --- | --- | --- |
| api | `GET /healthz` on `http`, every 2 s, 30 failures allowed (60 s) | `GET /healthz`, every 20 s, default 3 failures | `GET /readyz`, every 5 s, 2 failures |
| worker | none | `GET /healthz` on `metrics`, every 20 s | `GET /readyz` on `metrics`, every 10 s |
| postgres | none | `pg_isready`, every 20 s, 6 failures | `pg_isready`, every 5 s |

The api readiness only gates the Service. The worker has no Service, so its readiness is informational (rollout progress only).

## 5. Graceful shutdown

**Verified** (`cmd/gitorgasm/main.go`):

1. `signal.NotifyContext` cancels the root context on SIGTERM or SIGINT.
2. `serve` calls `http.Server.Shutdown` on every server. New connections stop. In-flight requests finish.
3. The worker calls `river.Client.Stop`. It stops fetching jobs and waits for running jobs. Jobs not finished in time are retried later (comment in `runWorker`).
4. The tracer provider flushes spans.
5. All steps share one deadline `SHUTDOWN_TIMEOUT` (default `25s`, `internal/config/config.go`).

**Verified** (manifests): `terminationGracePeriodSeconds` is 30 on `api` and 40 on `worker`. Both exceed `SHUTDOWN_TIMEOUT=25s` (ConfigMap `gitorgasm-core`). The worker gets more time because running jobs may need the full 25 s plus River shutdown.

## 6. Dashboards and alerts (Planned, Stage 1.5 / 6)

Grafana dashboard `Gitorgasm` (provision through the cluster repository like other dashboards):

- API request rate and error ratio by route.
- API p50 / p95 / p99 latency by route.
- GitHub requests by operation and status, and rate-limit remaining.
- Pod memory against limit (`container_memory_working_set_bytes`, from cAdvisor), and `go_memstats_heap_inuse_bytes`.
- Loki panel: `{namespace="gitorgasm"} | json | level="ERROR"`.

Alert rules (PromQL examples; thresholds are starting points, not measurements):

```promql
# API 5xx ratio above 5% for 10 minutes
sum(rate(gitorgasm_http_requests_total{status=~"5.."}[5m]))
  / sum(rate(gitorgasm_http_requests_total[5m])) > 0.05

# API p95 latency above 1 s for 10 minutes (excluding webhooks and health)
histogram_quantile(0.95, sum by (le) (rate(gitorgasm_http_request_duration_seconds_bucket{route!~".*(healthz|readyz).*"}[5m]))) > 1

# GitHub rate-limit budget low
min(gitorgasm_github_rate_limit_remaining) < 500

# GitHub errors (5xx or transport) dominate
sum(rate(gitorgasm_github_requests_total{status=~"5xx|error"}[10m]))
  / sum(rate(gitorgasm_github_requests_total[10m])) > 0.2

# Fewer than two scraped Gitorgasm processes (api + worker) are up
(count(up{namespace="gitorgasm"} == 1) or vector(0)) < 2

# Memory near the limit (GOMEMLIMIT is 200MiB of 256Mi)
max(container_memory_working_set_bytes{namespace="gitorgasm", container=~"api|worker"}) > 230 * 1024 * 1024
```

**Assumption**: the `namespace` label exists on scraped series. It depends on relabel rules in `k8s/monitoring/prometheus-configmap.yaml`. Pods carry `app.kubernetes.io/name` (`api`, `worker`), not `app`. Check which pod labels the relabel rules copy before you create per-process alerts.

After River job metrics land (Stage 2): alert when `gitorgasm_jobs_available` grows for 30 minutes, when `discarded` outcomes appear, and when `time() - gitorgasm_sync_last_success_timestamp_seconds > 2 * 6h` (two reconcile intervals). The backup staleness alerts already exist in the cluster (`cluster-audit.md` §6) and cover the Gitorgasm target once it is added.
