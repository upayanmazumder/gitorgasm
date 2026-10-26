//! Gitorgasm contributor scoring engine.
//!
//! A port of the legacy GH Org Tool scoring (`lib/scoring`). It turns contribution signals and a
//! scoring preset into contributor, repository, and team leaderboards. The crate is pure
//! computation: it has no I/O, database, or HTTP code. Golden tests prove that it reproduces
//! legacy scores exactly. See `docs/adr/0003-go-core-rust-for-scoring.md`.

mod engine;
mod model;

pub use engine::{ScoringError, contributor_leaderboard, repository_leaderboard, team_leaderboard};
pub use model::*;
