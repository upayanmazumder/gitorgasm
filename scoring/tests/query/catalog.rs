use std::num::NonZeroUsize;

use gitorgasm_scoring::{ContributorScore, ScoreQuery, ScoringCatalog, ScoringError, SignalType};

use super::{assert_score, signal};

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
