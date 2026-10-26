//! Bit-exact legacy scoring fixtures from `tests/golden/generate.ts`.

use std::fs;
use std::path::Path;

use gitorgasm_scoring::{
    ContributorScore, RepositoryScore, Ruleset, Signal, TeamMembership, TeamScore, contributor_leaderboard,
    repository_leaderboard, team_leaderboard,
};
use serde::Deserialize;

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct Fixture {
    ruleset: Ruleset,
    signals: Vec<Signal>,
    team_memberships: Vec<TeamMembership>,
    expected: Expected,
}

#[derive(Deserialize)]
struct Expected {
    contributors: Vec<ContributorScore>,
    repositories: Vec<RepositoryScore>,
    teams: Vec<TeamScore>,
}

#[test]
fn matches_legacy_engine() {
    let dir = Path::new(env!("CARGO_MANIFEST_DIR")).join("tests/golden");
    let mut checked = 0;
    for entry in fs::read_dir(&dir).unwrap() {
        let path = entry.unwrap().path();
        if path.extension().and_then(|e| e.to_str()) != Some("json") {
            continue;
        }
        let name = path.file_name().unwrap().to_string_lossy().into_owned();
        let f: Fixture = serde_json::from_slice(&fs::read(&path).unwrap()).unwrap();

        // Exact f64 equality: the port performs the same operations in the same order.
        assert_eq!(
            contributor_leaderboard(&f.signals, &f.ruleset).unwrap(),
            f.expected.contributors,
            "{name}: contributors"
        );
        assert_eq!(
            repository_leaderboard(&f.signals, &f.ruleset).unwrap(),
            f.expected.repositories,
            "{name}: repositories"
        );
        assert_eq!(
            team_leaderboard(&f.signals, &f.ruleset, &f.team_memberships).unwrap(),
            f.expected.teams,
            "{name}: teams"
        );
        checked += 1;
    }
    assert!(checked >= 6, "expected the generated fixtures, found {checked}");
}

#[test]
fn legacy_default_matches_fixture_ruleset() {
    let path = Path::new(env!("CARGO_MANIFEST_DIR")).join("tests/golden/default-ruleset.json");
    let f: Fixture = serde_json::from_slice(&fs::read(path).unwrap()).unwrap();
    assert_eq!(Ruleset::legacy_default(), f.ruleset);
}
