//! JSON stdin/stdout adapter for the organization scoring engine.

use std::error::Error;
use std::io::{self, Write};

use gitorgasm_scoring::{ScoreQuery, ScoringCatalog, Signal, TeamMembership};
use serde::Deserialize;

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Request {
    organization: String,
    signals: Vec<Signal>,
    memberships: Vec<TeamMembership>,
    query: ScoreQuery,
}

fn run() -> Result<(), Box<dyn Error>> {
    let mut args = std::env::args().skip(1);
    let first = args.next();
    match (first.as_deref(), args.next()) {
        (Some("--validate-query"), None) => {
            let query: ScoreQuery = serde_json::from_reader(io::stdin().lock())?;
            query.validate()?;
            return Ok(());
        }
        (None, None) => {}
        _ => return Err("usage: score [--validate-query]".into()),
    }
    let request: Request = serde_json::from_reader(io::stdin().lock())?;
    let mut catalog = ScoringCatalog::default();
    catalog.replace_organization(&request.organization, request.signals, request.memberships)?;
    let scores = catalog.query(&request.organization, &request.query)?;
    let mut stdout = io::stdout().lock();
    serde_json::to_writer(&mut stdout, scores)?;
    stdout.write_all(b"\n")?;
    Ok(())
}

fn main() {
    if let Err(error) = run() {
        eprintln!("score: {error}");
        std::process::exit(1);
    }
}
