# ADR 0009: Webhooks as hints plus periodic reconciliation

- Status: Accepted
- Date: 2026-10-26

## Context

GitHub webhooks can arrive twice, out of order, or not at all. GitHub does not redeliver failed deliveries automatically ([Redelivering webhooks](https://docs.github.com/en/webhooks/testing-and-troubleshooting-webhooks/redelivering-webhooks)). GitHub expects a 2XX response within 10 seconds ([Best practices](https://docs.github.com/en/webhooks/using-webhooks/best-practices-for-using-webhooks)). Webhook payloads have no timestamp for replay protection.

## Decision

- Verify `X-Hub-Signature-256` (HMAC-SHA256, constant-time) ([Validating webhook deliveries](https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries)). Cap the body at 25 MB.
- In one transaction: store the raw body in `webhook_deliveries` (primary key = `X-GitHub-Delivery`) and insert a `process_webhook` job. Return 202.
- Treat events as hints. Handlers enqueue a sync that re-reads GitHub. They do not apply payload state directly (exceptions: installation deletion and authorization revocation, which remove access).
- Run `reconcile_installations` every 6 h and when the worker starts. It enqueues a sync for every active installation.
- Replay protection = signature + delivery id deduplication.

Implementation: **Verified** `services/core/internal/httpapi/server.go` (route), `internal/httpapi/orgs.go` (handler, `maxWebhookBytes = 25 << 20`), `internal/jobs/jobs.go`.

## Consequences

- Duplicates and reordering converge to GitHub's state.
- Missed events are corrected within one reconcile interval.
- Each event costs a sync (more GitHub requests than applying the payload). Unique jobs collapse bursts.
- Raw bodies in the database need a retention job (**Planned**).

## Alternatives

- Apply payloads directly as the source of truth: rejected. Ordering and loss make state drift.
- Polling only: rejected. Slower updates and more API cost.
