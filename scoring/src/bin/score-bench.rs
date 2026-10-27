//! Deterministic, real 100,000-event scoring benchmark with a time limit.

use std::error::Error;
use std::time::{Duration, Instant};

use gitorgasm_scoring::{Metadata, ScoreQuery, ScoringCatalog, Signal, SignalType};

const EVENTS: usize = 100_000;

fn run() -> Result<(), Box<dyn Error>> {
    let mut args = std::env::args().skip(1);
    let limit = match (args.next().as_deref(), args.next(), args.next()) {
        (None, None, None) => 60,
        (Some("--limit-seconds"), Some(seconds), None) => seconds.parse::<u64>()?,
        _ => return Err("usage: score-bench [--limit-seconds 60]".into()),
    };
    if limit == 0 {
        return Err("--limit-seconds must be positive".into());
    }
    let kinds = [
        SignalType::Commit,
        SignalType::PrOpen,
        SignalType::PrMerge,
        SignalType::Review,
        SignalType::IssueOpen,
        SignalType::IssueClose,
    ];
    let mut signals = Vec::with_capacity(EVENTS);
    for i in 0..EVENTS {
        // Twenty-eight days and stable hour/second distribution; no clock or random input.
        let day = i % 28 + 1;
        let hour = i / 28 % 24;
        let second = i / (28 * 24) % 60;
        signals.push(Signal {
            user_id: i64::try_from(i % 200).expect("the remainder is below 200") + 1,
            user_login: format!("user-{}", i % 200 + 1),
            kind: kinds[(i / 200) % kinds.len()],
            repo: format!("repo-{}", (i / 200) % 40 + 1),
            event_timestamp: format!("2026-01-{day:02}T{hour:02}:00:{second:02}Z"),
            metadata: Metadata::default(),
        });
    }
    let mut catalog = ScoringCatalog::default();
    catalog.replace_organization("bench", signals, vec![])?;
    let started = Instant::now();
    let scores = catalog.query("bench", &ScoreQuery::default())?;
    let elapsed = started.elapsed();
    println!(
        "events={EVENTS} contributors={} repositories={} teams={} elapsed_seconds={:.3}",
        scores.contributors.len(),
        scores.repositories.len(),
        scores.teams.len(),
        elapsed.as_secs_f64()
    );
    if elapsed > Duration::from_secs(limit) {
        return Err(format!("scoring exceeded {limit}s limit: {elapsed:?}").into());
    }
    Ok(())
}

fn main() {
    if let Err(error) = run() {
        eprintln!("score-bench: {error}");
        std::process::exit(1);
    }
}
