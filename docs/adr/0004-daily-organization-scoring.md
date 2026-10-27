# ADR 0004: Daily organization scoring

- Status: Accepted
- Supersedes: [ADR 0003](0003-go-core-rust-for-scoring.md)

## Decision

Keep Go, River, PostgreSQL, and the Rust scoring crate. Do not add another service or database.

### Ingestion

- A successful repository sync enqueues `ingest_contributions` in the same transaction. A run starts at most once per UTC day for each installation.
- A run records progress per repository: the phase (commits, pulls, issues) and the next page URL. Each page and its cursor are stored in one transaction. A rate limit, a timeout, or a worker restart resumes the same run. Finished repositories are not fetched again.
- One attempt stops after 40 minutes and snoozes. River rescues jobs after one hour, so an attempt never overlaps a rescue.
- Each repository has a watermark (`repository_ingest_state`). Without one, the run reads the full GitHub history of that repository. With one, commits are read again for 30 days and other activity for 1 hour before the watermark.
- Pull requests have no `since` parameter. The run reads them by update time, newest first, and stops at the first one older than the window.
- A repository that returns 404 is skipped and keeps its stored events. A 409 on commits means the repository is empty. A 410 on issue events means the issue was deleted.
- Publish runs in one transaction. Commits are replaced inside each finished repository's commit window. Other events are upserted by event ID. Then the watermarks advance, the generation increases, and the scoring job is inserted.
- A failure other than a rate limit records `last_failed_at`. Scores then report `fresh: false` until the next publish.

### Scoring

- The worker runs the Rust `score` binary as a local process on the `scoring` queue with one slot. It stores the default scores only if the generation is unchanged.
- The API runs at most two score processes and waits two seconds for a free slot. A custom query must finish in 25 seconds. It reads events in a read-only transaction with a 20 second statement timeout.
- The binary exits with code 2 for an invalid query or input. The API maps that to 400. Other exits are server errors. Only the first line of stderr, up to 300 bytes, reaches logs.
- Custom results are cached per generation and canonical query. At most 200 cached queries are kept per generation. A new generation removes older entries.
- Named periods without `as_of` resolve against the latest ingest time. Saved presets never store `as_of`. An organization keeps at most 50 presets.

The App needs read access to repository Contents, Pull requests, and Issues.

## Evidence and limits

- `make score-bench` scores 100,000 events in about 0.05 seconds on the developer workstation, with 34 MB peak memory. The command fails above 60 seconds. This figure excludes GitHub requests, database reads, and process start.
- GitHub has no complete organization-wide contribution feed, so ingest uses per-repository REST pagination. The first run of a large organization can take many attempts because of rate limits. Each attempt keeps its progress.
- Reviews and issue events are fetched for each changed pull request or issue. Their cost grows with activity in the window, not with history.
- Teams are empty until member data is mirrored.
