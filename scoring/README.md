# Scoring

This Rust crate scores a complete snapshot of one organization's GitHub contributions. It does not fetch contributions or persist them. The caller must provide the organization's complete, fresh signal snapshot and memberships; it must not mix organizations. Re-run the scorer on each refreshed snapshot rather than adding scores from earlier runs. Results are deterministic for the same signals, memberships, and query; no implicit current time is used.

## Default scoring

| Event type | Points | Daily per-user limit |
| --- | ---: | ---: |
| `commit` | 1 | 10 |
| `pr_open` | 4 | 10 |
| `pr_merge` | 4 | 10 |
| `review` | 3 | 10 |
| `issue_open` | 2 | 10 |
| `issue_close` | 1 | 10 |
| `pr_close_no_merge`, `review_comment`, `comment`, `spam` | 0 | none |

Each eligible event receives its type's base points. Bot activity (`metadata.isBot: true`) and self-reviews (`metadata.isSelfReview: true` on reviews) receive zero points, but appear in event counts. An omitted flag is false. The ingestion source must determine the actor and flags; the scorer does not infer them from message text. `pr_open` and `pr_merge` are two distinct contributions, each scored once when the upstream source emits them. An issue close is credited to the actor represented by the signal's user ID. No linked-PR metadata is required to score it. Review states `approved`, `changes_requested`, and `commented` have default weight 1; absent state means `commented`, and unknown states have weight 1. Commit addition/deletion counts are reported as metadata, never used to increase points.

The first 10 eligible events **per user, type, UTC day** earn points. Further events that day remain counted but score zero. Among events that pass that limit, the first 20 **per user, type, ISO UTC week** receive full points. The 21st receives 95%, then 90%, etc., to a floor of 25%. Weekly counts reset at Monday 00:00 UTC; daily limits reset at UTC midnight. Signals are scored in timestamp order (equal timestamps retain input order). Flags that zero an event do not consume a quota. Points are attributed once to their contributor, repository, and each qualifying team; repositories do not independently re-score events. A team may restrict its events to named repositories. Duplicate team memberships do not duplicate events. Teams with no selected signals retain zero rows. Sort is descending by score, stable on ties; rank is one-based and distinct even for ties.

For custom `rules`, base points are per-type entries, later duplicate entries override earlier ones; zero-point conditions and daily quotas also use the last entry. Nonnegative finite base points, weights, multipliers, decay factors and floor fractions are required, with decay and floor at most 1. Penalties may be negative. For each qualifying event, compute `base × max(floorFraction, 1 − max(0, weeklyOccurrence − weeklyThreshold) × decayFactor) × applicable multipliers × review state weight`. `first_activity` applies once per scored user/type/day; `merged_pr_commit` requires `metadata.isInMergedPR`, and `pr_linked_to_issue` requires a PR merge with `metadata.hasLinkedIssue`. Applied custom multiplier factors multiply together. `spamPenalty` and `prClosedNoMergePenalty` apply to their event types unless excluded by enabled zero-point conditions. The row `score` is the sum of category points and penalties, floored at zero. Category breakdowns, counts and `additions`/`deletions` are emitted alongside it. Default multipliers and penalties are zero/empty, so unobserved metadata does not confer points. `Ruleset::balanced_default()` is the canonical default; `ScoreQuery::default()` selects it.

## CLI JSON contract

From `scoring/`, run `cargo run --bin score < request.json > scores.json`, or build with `cargo build --bin score` and invoke `target/debug/score`. Input is **one** JSON object on stdin with all four top-level fields:

```json
{
  "organization": "12345",
  "signals": [
    {"userId": 42, "userLogin": "alice", "type": "pr_open", "repo": "api", "eventTimestamp": "2026-01-05T10:00:00Z", "metadata": {"isBot": false}}
  ],
  "memberships": [
    {"userId": 42, "teamSlug": "backend", "teamName": "Backend", "repositoryNames": ["api"]}
  ],
  "query": {}
}
```

`organization` is a nonblank string (an installation ID or account login); `signals` is the complete selected organization snapshot, not incremental point totals; `memberships` may be `[]`. Each signal requires integer `userId`, string `userLogin`, `type`, `repo`, and RFC 3339 `eventTimestamp`. Types are `commit`, `pr_open`, `pr_merge`, `pr_close_no_merge`, `issue_open`, `issue_close`, `review`, `review_comment`, `comment`, or `spam`. `metadata` is optional; fields are optional `isBot`, `isSelfReview`, `isSelfMerge`, `isInMergedPR`, `hasLinkedIssue`, `hasLinkedPR` booleans, `state` string, and `additions`/`deletions` signed integers. Metadata is scoring flags and line counts only; no message/body/content is needed. Each membership requires integer `userId`, string `teamSlug`, `teamName`, and optionally `repositoryNames` (array of repository names or null). If *any* membership of a team has listed repositories, its team score includes only those repositories; otherwise it includes all repos.

`query: {}` selects all events with the balanced default rules. It can contain:

```json
{
  "period": {"kind": "custom", "start": "2026-01-01T00:00:00Z", "end": "2026-02-01T00:00:00Z"},
  "filter": {"preset": "code", "users": [42], "repositories": ["api"], "kinds": ["commit", "pr_open"], "excludeBots": true, "search": "ali"}
}
```

A period is `{ "kind": "all" }`, `{ "kind": "custom", "start": RFC3339, "end": RFC3339 }`, or `{ "kind": "preset", "preset": NAME, "as_of": RFC3339 }`. Preset names: `today`, `this_week`, `this_month`, `last7_days`, `last30_days`. `as_of` is **required**, is the exclusive end, and eliminates clock-dependent cache keys. Every bounded period includes its start and excludes its end; invalid or zero-width bounds fail. Note `as_of` is snake_case inside `period`, while filter and signal fields are camelCase. All optional filter fields default to no restriction. Preset is `all`, `code` (commits and PRs), `reviews` (reviews and comments), or `issues`. Lists intersect with the preset and each other. `search` is a trimmed ASCII case-insensitive substring of a login **or** repository name. Query may also contain `rules` as a complete camelCase `Ruleset` JSON object with `basePoints`, `multipliers`, `reviewStateWeights`, `dailyQuotas`, `diminishingReturns` (`weeklyThreshold`, `decayFactor`, `floorFraction`), `zeroPointConditions`, `spamPenalty`, and `prClosedNoMergePenalty`. Omit `rules` to use all defaults; specifying it requires the complete object. Rule entries use `type` for contribution types, `kind` for multiplier/zero-point kinds, and `appliesTo` for multiplier signal types.

Stdout contains a single `Scoreboards` JSON object `{ "contributors": [...], "repositories": [...], "teams": [...] }`. Contributor rows contain `userId`, `userLogin`; repository rows `repoName`; team rows `teamSlug`, `teamName`, `memberCount`. Every row also contains `rank`, `score`, `breakdown` (`commits`, `pullRequests`, `reviews`, `reviewComments`, `issues`, `comments`, `penalties`), `counts` (same categories except penalties), `additions`, and `deletions`. Empty memberships produce `teams: []`. Invalid JSON, incomplete envelopes, unsupported event types, invalid timestamps/rules/periods, or empty organization names exit with code 2 and an error on stderr, not success JSON. I/O and other internal failures exit with code 1.

To validate a saved query without scoring a snapshot, run `cargo run --bin score -- --validate-query < query.json`. This mode reads a **bare** `ScoreQuery` object (for example `{}` or the query example above), writes no stdout on success, and exits with code 2 on invalid JSON, period, filter enum, or rules. `ScoreQuery::validate()` exposes the same validation to Rust callers.

## Query API, presets and cache

`ScoringCatalog::replace_organization(name, signals, memberships)` replaces that organization's snapshot and invalidates only its cached results. `catalog.query(name, &ScoreQuery::default())` scores the default; custom periods use `Period::Custom { start, end }` and relative periods use `Period::Preset { preset, as_of }`. Use `SignalFilter` for user/repository/type selection, bot exclusion, and search. `save_preset(name, preset_name, query)` and `query_preset(name, preset_name)` retain named options per organization in the catalog process. They are not exposed by the one-shot CLI and are not persistent storage.

Each organization has a bounded FIFO result cache, default capacity 32 complete queries; use `ScoringCatalog::new(nonzero_capacity)` to customize. An exact query cache hit returns the prior result. Replacing a snapshot or `clear_cache(name)` invalidates it; `cache_stats(name)` exposes hits, successful misses, and entries. Cache lifetime is the catalog process. A one-shot CLI request owns a fresh catalog and never reuses previous request results.

## Deterministic scale check

`cargo run --release --bin score-bench -- --limit-seconds 60` scores 100,000 generated events with the real catalog and exits nonzero if scoring time exceeds 60 seconds. It reports real contributor/repository/team counts and measured elapsed seconds; there is no precomputed answer. The generated input and event order are deterministic. The timer measures scoring, not generation or compilation. Integration tests: `cargo test --locked` from `scoring/`.
