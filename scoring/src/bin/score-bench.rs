//! Deterministic, real 100,000-event scoring benchmark with a time limit.

mod benchmark;

fn main() {
    if let Err(error) = benchmark::run() {
        eprintln!("score-bench: {error}");
        std::process::exit(1);
    }
}
