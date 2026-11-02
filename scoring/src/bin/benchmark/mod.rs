use std::error::Error;
use std::time::{Duration, Instant};

use gitorgasm_scoring::{ScoreQuery, ScoringCatalog};

mod workload;

use workload::EVENTS;

pub(super) fn run() -> Result<(), Box<dyn Error>> {
    let mut args = std::env::args().skip(1);
    let limit = match (args.next().as_deref(), args.next(), args.next()) {
        (None, None, None) => 60,
        (Some("--limit-seconds"), Some(seconds), None) => seconds.parse::<u64>()?,
        _ => return Err("usage: score-bench [--limit-seconds 60]".into()),
    };
    if limit == 0 {
        return Err("--limit-seconds must be positive".into());
    }
    let mut catalog = ScoringCatalog::default();
    catalog.replace_organization("bench", workload::signals(), vec![])?;
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
