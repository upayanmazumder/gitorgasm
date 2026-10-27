# ADR 0004: Daily organization scoring

- Status: Accepted
- Supersedes: [ADR 0003](0003-go-core-rust-for-scoring.md)

## Decision

Keep Go, River, PostgreSQL, and the Rust scoring crate. Do not add another service or database.

After a successful repository sync, River requests contribution ingestion. The ingestion job runs once per organization each UTC day. It records GitHub events in a private run table. It publishes events only after every repository and page succeeds. It then inserts the scoring job in the same database transaction.

The scoring job reads the completed events and runs the Rust scorer as a local process. It stores the new default scores only if the ingestion generation still matches. A failed fetch or score calculation leaves the previous scores unchanged. Custom score requests use the same scorer. Their database cache includes the ingestion generation and all query options.

The App needs read access to repository Contents, Pull requests, and Issues. GitHub App permissions need a separate owner action. The code does not change live App settings.

## Evidence and limits

The local `make score-bench` command processed 100,000 events in 0.052 seconds on the developer workstation. The command fails if scoring takes longer than 60 seconds. This number excludes GitHub requests, database reads, and process start. The source uses REST pagination because GitHub does not offer a complete organization-wide contribution feed through GraphQL. Large organizations can still hit GitHub rate limits during their first full history scan. River retries these failures without publishing partial results.

Custom time periods include the start and exclude the end. A failed run marks scores as old without exposing staged events. Organization reads check the user's installation grant before returning scores or presets.
