//! Organization, period, filter, preset, and cache behavior.

use std::num::NonZeroUsize;

use chrono::{DateTime, Utc};
use gitorgasm_scoring::{
    ContributorScore, FilterPreset, Metadata, Period, PeriodPreset, ScoreQuery, ScoringCatalog, ScoringError, Signal,
    SignalFilter, SignalType, TeamMembership,
};

fn instant(value: &str) -> DateTime<Utc> {
    DateTime::parse_from_rfc3339(value).unwrap().to_utc()
}

fn signal(user: i64, login: &str, repo: &str, kind: SignalType, at: &str) -> Signal {
    Signal {
        user_id: user,
        user_login: login.into(),
        repo: repo.into(),
        kind,
        event_timestamp: at.into(),
        metadata: Metadata::default(),
    }
}

fn assert_score(actual: f64, expected: f64) {
    assert_eq!(actual.to_bits(), expected.to_bits());
}

#[test]
fn organizations_are_isolated_and_replacement_invalidates_only_their_cache() {
    let mut catalog = ScoringCatalog::new(NonZeroUsize::new(2).unwrap());
    let at = "2026-01-06T10:00:00Z";
    catalog
        .replace_organization("north", vec![signal(1, "alice", "api", SignalType::Commit, at)], vec![])
        .unwrap();
    catalog
        .replace_organization("south", vec![signal(2, "bob", "web", SignalType::Review, at)], vec![])
        .unwrap();
    let query = ScoreQuery::default();
    assert_eq!(
        catalog.query("north", &query).unwrap().contributors[0].user_login,
        "alice"
    );
    assert_eq!(
        catalog.query("south", &query).unwrap().contributors[0].user_login,
        "bob"
    );
    assert_eq!(catalog.cache_stats("south").unwrap().misses, 1);
    assert_score(
        catalog.query("north", &query).unwrap().contributors[0].totals.score,
        1.0,
    );
    assert_eq!(catalog.cache_stats("north").unwrap().hits, 1);

    catalog
        .replace_organization("north", vec![signal(3, "carol", "api", SignalType::Commit, at)], vec![])
        .unwrap();
    assert_eq!(
        catalog.query("north", &query).unwrap().contributors[0].user_login,
        "carol"
    );
    assert_eq!(catalog.cache_stats("north").unwrap().misses, 2);
    assert_eq!(
        catalog.query("south", &query).unwrap().contributors[0].user_login,
        "bob"
    );
    assert_eq!(catalog.cache_stats("south").unwrap().hits, 1);
    assert!(matches!(
        catalog.query("missing", &query),
        Err(ScoringError::UnknownOrganization(_))
    ));
}

#[test]
fn custom_and_relative_periods_use_utc_half_open_boundaries() {
    let mut catalog = ScoringCatalog::default();
    catalog
        .replace_organization(
            "org",
            vec![
                signal(1, "before", "repo", SignalType::Commit, "2026-01-04T23:59:59Z"),
                signal(2, "start", "repo", SignalType::Commit, "2026-01-05T02:00:00+02:00"),
                signal(3, "inside", "repo", SignalType::Commit, "2026-01-07T23:59:59Z"),
                signal(4, "end", "repo", SignalType::Commit, "2026-01-08T00:00:00Z"),
            ],
            vec![],
        )
        .unwrap();
    let mut query = ScoreQuery::default();
    let end = instant("2026-01-08T00:00:00Z");
    query.period = Period::Custom {
        start: instant("2026-01-05T00:00:00Z"),
        end,
    };
    let names: Vec<_> = catalog
        .query("org", &query)
        .unwrap()
        .contributors
        .iter()
        .map(|r| r.user_login.as_str())
        .collect();
    assert_eq!(names, ["start", "inside"]);
    query.period = Period::Preset {
        preset: PeriodPreset::ThisWeek,
        as_of: end,
    };
    assert_eq!(catalog.query("org", &query).unwrap().contributors.len(), 2);
    query.period = Period::Preset {
        preset: PeriodPreset::Last7Days,
        as_of: end,
    };
    assert_eq!(catalog.query("org", &query).unwrap().contributors.len(), 3);
    query.period = Period::Custom { start: end, end };
    assert_eq!(catalog.query("org", &query), Err(ScoringError::InvalidPeriod));
}

#[test]
fn filters_search_and_rules_compose_before_scoring() {
    let mut catalog = ScoringCatalog::default();
    let at = "2026-01-06T10:00:00Z";
    let mut bot = signal(3, "build-bot", "api", SignalType::Commit, at);
    bot.metadata.is_bot = Some(true);
    catalog
        .replace_organization(
            "org",
            vec![
                signal(1, "Alice", "api", SignalType::Commit, at),
                signal(1, "Alice", "api", SignalType::Review, "2026-01-06T11:00:00Z"),
                signal(2, "bob", "web", SignalType::Commit, at),
                bot,
            ],
            vec![],
        )
        .unwrap();
    let mut query = ScoreQuery {
        filter: SignalFilter {
            preset: FilterPreset::Code,
            users: vec![1, 2, 3],
            repositories: vec!["api".into()],
            kinds: vec![SignalType::Commit],
            exclude_bots: true,
            search: Some("ALIce".into()),
        },
        ..ScoreQuery::default()
    };
    let rows = &catalog.query("org", &query).unwrap().contributors;
    assert_eq!(rows.len(), 1);
    assert_eq!(rows[0].user_login, "Alice");
    assert_score(rows[0].totals.score, 1.0);
    query
        .rules
        .base_points
        .iter_mut()
        .find(|r| r.kind == SignalType::Commit)
        .unwrap()
        .points = 8.0;
    assert_score(catalog.query("org", &query).unwrap().contributors[0].totals.score, 8.0);
    query.filter.search = Some("API".into());
    assert_eq!(
        catalog.query("org", &query).unwrap().contributors[0].user_login,
        "Alice"
    );
    query.filter.preset = FilterPreset::Reviews;
    assert_eq!(
        catalog.query("org", &query).unwrap().contributors,
        [] as [ContributorScore; 0]
    );
}

#[test]
fn named_presets_are_org_scoped_and_custom_options_remain_mutable() {
    let mut catalog = ScoringCatalog::default();
    let at = "2026-01-06T10:00:00Z";
    for org in ["north", "south"] {
        catalog
            .replace_organization(org, vec![signal(1, "alice", "api", SignalType::Commit, at)], vec![])
            .unwrap();
    }
    let mut custom = ScoreQuery::default();
    custom.rules.base_points[0].points = 4.0;
    catalog.save_preset("north", "small", custom.clone()).unwrap();
    assert_score(
        catalog.query_preset("north", "small").unwrap().contributors[0]
            .totals
            .score,
        4.0,
    );
    assert!(matches!(
        catalog.query_preset("south", "small"),
        Err(ScoringError::UnknownPreset(_))
    ));
    custom.rules.base_points[0].points = 8.0;
    catalog.save_preset("north", "small", custom).unwrap();
    assert_score(
        catalog.query_preset("north", "small").unwrap().contributors[0]
            .totals
            .score,
        8.0,
    );
}

#[test]
fn bounded_cache_reuses_exact_queries_and_evicts_old_entries() {
    let mut catalog = ScoringCatalog::new(NonZeroUsize::new(1).unwrap());
    catalog
        .replace_organization(
            "org",
            vec![signal(1, "alice", "api", SignalType::Commit, "2026-01-06T10:00:00Z")],
            vec![],
        )
        .unwrap();
    let all = ScoreQuery::default();
    assert_score(catalog.query("org", &all).unwrap().contributors[0].totals.score, 1.0);
    assert_score(catalog.query("org", &all).unwrap().contributors[0].totals.score, 1.0);
    let mut filtered = all.clone();
    filtered.filter.repositories = vec!["other".into()];
    assert_eq!(
        catalog.query("org", &filtered).unwrap().contributors,
        [] as [ContributorScore; 0]
    );
    assert_score(catalog.query("org", &all).unwrap().contributors[0].totals.score, 1.0);
    let stats = catalog.cache_stats("org").unwrap();
    assert_eq!((stats.hits, stats.misses, stats.entries), (1, 3, 1));
}

#[test]
fn team_scores_use_filtered_signals_but_retain_membership_rows() {
    let mut catalog = ScoringCatalog::default();
    let at = "2026-01-06T10:00:00Z";
    let member = |user_id, team_slug: &str| TeamMembership {
        user_id,
        team_slug: team_slug.into(),
        team_name: team_slug.into(),
        repository_names: None,
    };
    catalog
        .replace_organization(
            "org",
            vec![
                signal(1, "alice", "api", SignalType::Commit, at),
                signal(2, "bob", "web", SignalType::Commit, at),
            ],
            vec![member(1, "shared"), member(2, "shared"), member(2, "web-only")],
        )
        .unwrap();
    let mut query = ScoreQuery::default();
    query.filter.repositories.push("api".into());
    let teams = &catalog.query("org", &query).unwrap().teams;
    assert_score(
        teams
            .iter()
            .find(|team| team.team_slug == "shared")
            .unwrap()
            .totals
            .score,
        1.0,
    );
    assert_score(
        teams
            .iter()
            .find(|team| team.team_slug == "web-only")
            .unwrap()
            .totals
            .score,
        0.0,
    );
    assert_eq!(
        teams
            .iter()
            .find(|team| team.team_slug == "shared")
            .unwrap()
            .member_count,
        2
    );
}

#[test]
fn invalid_input_cannot_poison_cache_and_manual_invalidation_recomputes() {
    let mut catalog = ScoringCatalog::default();
    catalog
        .replace_organization(
            "org",
            vec![signal(1, "alice", "api", SignalType::Commit, "invalid")],
            vec![],
        )
        .unwrap();
    let mut query = ScoreQuery::default();
    assert_eq!(
        catalog.query("org", &query),
        Err(ScoringError::InvalidTimestamp("invalid".into()))
    );
    assert_eq!(catalog.cache_stats("org").unwrap().entries, 0);
    query.filter.repositories.push("other".into());
    assert_eq!(
        catalog.query("org", &query).unwrap().contributors,
        [] as [ContributorScore; 0]
    );
    assert_eq!(catalog.cache_stats("org").unwrap().misses, 1);
    catalog.clear_cache("org").unwrap();
    assert_eq!(catalog.cache_stats("org").unwrap().entries, 0);
    assert_eq!(
        catalog.query("org", &query).unwrap().contributors,
        [] as [ContributorScore; 0]
    );
    assert_eq!(catalog.cache_stats("org").unwrap().misses, 2);
    assert_eq!(
        catalog.replace_organization(" ", vec![], vec![]),
        Err(ScoringError::InvalidOrganization)
    );
    assert_eq!(
        catalog.save_preset("org", " ", ScoreQuery::default()),
        Err(ScoringError::InvalidPreset)
    );
}

#[test]
fn query_json_uses_explicit_periods_and_defaults() {
    let default: ScoreQuery = serde_json::from_str("{}").unwrap();
    assert_eq!(default, ScoreQuery::default());
    let query: ScoreQuery = serde_json::from_str(
        r#"{
        "period": {"kind": "preset", "preset": "last30_days", "as_of": "2026-01-31T00:00:00Z"},
        "filter": {"preset": "issues", "excludeBots": true, "kinds": ["issue_open"]}
    }"#,
    )
    .unwrap();
    assert_eq!(query.filter.preset, FilterPreset::Issues);
    assert!(query.filter.exclude_bots);
    assert_eq!(query.filter.kinds, vec![SignalType::IssueOpen]);
    assert_eq!(
        serde_json::from_value::<ScoreQuery>(serde_json::to_value(query.clone()).unwrap()).unwrap(),
        query
    );
    assert!(serde_json::from_str::<ScoreQuery>(r#"{"period":{"kind":"preset","preset":"today"}}"#).is_err());
    assert!(serde_json::from_str::<ScoreQuery>(r#"{"filter":{"exclude_bots":true}}"#).is_err());
}
