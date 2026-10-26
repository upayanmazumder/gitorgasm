//! Pure, legacy-compatible contributor, repository, and team scoring.

mod engine;
mod error;
mod model;

pub use engine::{contributor_leaderboard, repository_leaderboard, team_leaderboard};
pub use error::ScoringError;
pub use model::*;
