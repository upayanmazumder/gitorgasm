use std::io::{self, Write};
use std::process::ExitCode;

use gitorgasm_scoring::{ScoreQuery, ScoringCatalog};

mod protocol;

use protocol::{Failure, Request};

fn execute() -> Result<(), Failure> {
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

pub(super) fn run() -> ExitCode {
    match execute() {
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
