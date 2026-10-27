//! JSON stdin/stdout adapter for the organization scoring engine.
//!
//! Exit codes: 0 success, 2 invalid input or query, 1 any other failure.

use std::io::{self, Write};
use std::process::ExitCode;

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

enum Failure {
    Input(String),
    Internal(String),
}

impl From<serde_json::Error> for Failure {
    fn from(error: serde_json::Error) -> Self {
        if error.is_io() {
            Self::Internal(error.to_string())
        } else {
            Self::Input(error.to_string())
        }
    }
}

impl From<gitorgasm_scoring::ScoringError> for Failure {
    fn from(error: gitorgasm_scoring::ScoringError) -> Self {
        Self::Input(error.to_string())
    }
}

impl From<io::Error> for Failure {
    fn from(error: io::Error) -> Self {
        Self::Internal(error.to_string())
    }
}

fn run() -> Result<(), Failure> {
    let mut args = std::env::args().skip(1);
    let first = args.next();
    match (first.as_deref(), args.next()) {
        (Some("--validate-query"), None) => {
            let query: ScoreQuery = serde_json::from_reader(io::stdin().lock())?;
            query.validate()?;
            return Ok(());
        }
        (None, None) => {}
        _ => return Err(Failure::Internal("usage: score [--validate-query]".into())),
    }
    let request: Request = serde_json::from_reader(io::BufReader::new(io::stdin().lock()))?;
    let mut catalog = ScoringCatalog::default();
    catalog.replace_organization(&request.organization, request.signals, request.memberships)?;
    let scores = catalog.query(&request.organization, &request.query)?;
    let mut stdout = io::BufWriter::new(io::stdout().lock());
    serde_json::to_writer(&mut stdout, scores)?;
    stdout.write_all(b"\n")?;
    stdout.flush()?;
    Ok(())
}

fn main() -> ExitCode {
    match run() {
        Ok(()) => ExitCode::SUCCESS,
        Err(Failure::Input(message)) => {
            eprintln!("score: invalid input: {message}");
            ExitCode::from(2)
        }
        Err(Failure::Internal(message)) => {
            eprintln!("score: {message}");
            ExitCode::FAILURE
        }
    }
}
