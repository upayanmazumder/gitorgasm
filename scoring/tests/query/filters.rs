use gitorgasm_scoring::{
    ContributorScore, FilterPreset, ScoreQuery, ScoringCatalog, SignalFilter, SignalType, TeamMembership,
};

use super::{assert_score, signal};

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
