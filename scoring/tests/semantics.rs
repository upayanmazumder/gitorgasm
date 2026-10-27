//! Checks scoring rules, shared totals, and team scope.
#![allow(clippy::float_cmp)]

use gitorgasm_scoring::{
    Metadata, Ruleset, ScoreQuery, ScoringCatalog, ScoringError, Signal, SignalType, TeamMembership,
};

fn signal(user_id: i64, kind: SignalType, repo: &str, timestamp: &str) -> Signal {
    Signal {
        user_id,
        user_login: format!("user-{user_id}"),
        kind,
        repo: repo.into(),
        event_timestamp: timestamp.into(),
        metadata: Metadata::default(),
    }
}

#[test]
fn balanced_defaults_reward_review_and_pr_without_double_scoring_across_views() {
    let at = "2026-01-05T10:00:00Z";
    let signals = vec![
        signal(1, SignalType::Commit, "api", at),
        signal(1, SignalType::PrOpen, "api", at),
        signal(1, SignalType::PrMerge, "api", at),
        signal(2, SignalType::Review, "api", at),
        signal(2, SignalType::IssueOpen, "api", at),
        signal(2, SignalType::IssueClose, "api", at),
    ];
    let membership = [1, 2].map(|user_id| TeamMembership {
        user_id,
        team_slug: "dev".into(),
        team_name: "Developers".into(),
        repository_names: None,
    });
    let mut catalog = ScoringCatalog::default();
    catalog.replace_organization("org", signals, membership.into()).unwrap();
    let scored = catalog.query("org", &ScoreQuery::default()).unwrap();
    assert_eq!(scored.contributors[0].totals.score, 9.0);
    assert_eq!(scored.contributors[1].totals.score, 6.0);
    assert_eq!(scored.repositories[0].totals.score, 15.0);
    assert_eq!(scored.teams[0].totals.score, 15.0);
    assert_eq!(scored.teams[0].member_count, 2);
    assert_eq!(scored.repositories[0].totals.counts.pull_requests, 2);
    assert_eq!(scored.repositories[0].totals.counts.issues, 2);
}

#[test]
fn bots_and_self_reviews_count_but_do_not_score() {
    let at = "2026-01-05T10:00:00Z";
    let mut bot = signal(1, SignalType::PrMerge, "api", at);
    bot.metadata.is_bot = Some(true);
    let mut review = signal(1, SignalType::Review, "api", at);
    review.metadata.is_self_review = Some(true);
    let mut catalog = ScoringCatalog::default();
    catalog.replace_organization("org", vec![bot, review], vec![]).unwrap();
    let totals = &catalog.query("org", &ScoreQuery::default()).unwrap().contributors[0].totals;
    assert_eq!(totals.score, 0.0);
    assert_eq!(totals.counts.pull_requests, 1);
    assert_eq!(totals.counts.reviews, 1);
}

#[test]
fn daily_quota_and_weekly_decay_follow_utc_calendar() {
    let mut signals = Vec::new();
    for day in 5..=7 {
        for hour in 0..10 {
            signals.push(signal(
                1,
                SignalType::Commit,
                "api",
                &format!("2026-01-{day:02}T{hour:02}:00:00Z"),
            ));
        }
    }
    signals.push(signal(1, SignalType::Commit, "api", "2026-01-07T11:00:00Z"));
    // Sunday offset is Monday UTC; it falls into the following UTC day/week.
    signals.push(signal(1, SignalType::Commit, "api", "2026-01-11T23:30:00-02:00"));
    let mut catalog = ScoringCatalog::default();
    catalog.replace_organization("org", signals, vec![]).unwrap();
    let row = &catalog.query("org", &ScoreQuery::default()).unwrap().contributors[0];
    // 20 full points, 10 decayed (0.95 down to 0.5), one new-week point.
    assert!((row.totals.score - 28.25).abs() < 1e-9);
    assert_eq!(row.totals.counts.commits, 32);
}

#[test]
fn rules_reject_non_finite_or_negative_inputs_and_scores_count_all_events() {
    let mut catalog = ScoringCatalog::default();
    let signals = vec![signal(1, SignalType::Commit, "api", "2026-01-05T10:00:00Z")];
    catalog.replace_organization("org", signals, vec![]).unwrap();
    let mut query = ScoreQuery::default();
    query.rules.base_points[0].points = -1.0;
    assert_eq!(
        catalog.query("org", &query),
        Err(ScoringError::InvalidRule("basePoints.points"))
    );
    query.rules.base_points[0].points = f64::INFINITY;
    assert_eq!(
        catalog.query("org", &query),
        Err(ScoringError::NonFiniteRule("basePoints.points"))
    );
    assert_eq!(catalog.cache_stats("org").unwrap().entries, 0);
    assert_eq!(Ruleset::balanced_default().base_points[0].points, 1.0);
}

#[test]
fn teams_apply_repository_scope_once_per_user() {
    let signals = vec![
        signal(1, SignalType::PrOpen, "api", "2026-01-05T10:00:00Z"),
        signal(1, SignalType::PrOpen, "web", "2026-01-05T11:00:00Z"),
    ];
    let membership = TeamMembership {
        user_id: 1,
        team_slug: "one".into(),
        team_name: "One".into(),
        repository_names: Some(vec!["api".into()]),
    };
    let mut catalog = ScoringCatalog::default();
    catalog
        .replace_organization("org", signals, vec![membership.clone(), membership])
        .unwrap();
    let scored = catalog.query("org", &ScoreQuery::default()).unwrap();
    assert_eq!(scored.contributors[0].totals.score, 8.0);
    assert_eq!(scored.teams[0].totals.score, 4.0);
    assert_eq!(scored.teams[0].member_count, 1);
}
