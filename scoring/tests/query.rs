//! Organization, period, filter, preset, and cache behavior.

use gitorgasm_scoring::{Metadata, Signal, SignalType};

#[path = "query/catalog.rs"]
mod catalog;
#[path = "query/filters.rs"]
mod filters;
#[path = "query/periods.rs"]
mod periods;

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
