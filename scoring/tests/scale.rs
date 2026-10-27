//! Checks that large contribution sets retain all scored events.

use gitorgasm_scoring::{Metadata, ScoreQuery, ScoringCatalog, Signal, SignalType};

#[test]
fn scores_ten_thousand_events_without_dropping_events_or_views() {
    let signals: Vec<Signal> = (0..10_000)
        .map(|i| Signal {
            user_id: i64::from(i % 100),
            user_login: format!("member-{}", i % 100),
            kind: SignalType::Commit,
            repo: format!("repo-{}", i % 20),
            event_timestamp: format!("2026-01-{:02}T{:02}:00:00Z", i / 1000 + 1, i / 100 % 10),
            metadata: Metadata::default(),
        })
        .collect();
    let mut catalog = ScoringCatalog::default();
    catalog.replace_organization("large", signals, vec![]).unwrap();
    let scores = catalog.query("large", &ScoreQuery::default()).unwrap();
    assert_eq!(scores.contributors.len(), 100);
    assert_eq!(scores.repositories.len(), 20);
    assert_eq!(scores.teams.len(), 0);
    assert_eq!(
        scores
            .contributors
            .iter()
            .map(|row| u64::from(row.totals.counts.commits))
            .sum::<u64>(),
        10_000
    );
    assert_eq!(
        scores
            .repositories
            .iter()
            .map(|row| u64::from(row.totals.counts.commits))
            .sum::<u64>(),
        10_000
    );
    assert_eq!(catalog.cache_stats("large").unwrap().entries, 1);
}
