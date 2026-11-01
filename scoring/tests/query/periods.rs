use chrono::{DateTime, Utc};
use gitorgasm_scoring::{Period, PeriodPreset, ScoreQuery, ScoringCatalog, ScoringError, SignalType};

use super::signal;

fn instant(value: &str) -> DateTime<Utc> {
    DateTime::parse_from_rfc3339(value).unwrap().to_utc()
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
