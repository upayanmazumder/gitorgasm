//! Pure, organization-scoped contributor, repository, and team scoring.

mod engine;
mod error;
mod model;
mod query;

pub use engine::{contributor_leaderboard, repository_leaderboard, team_leaderboard};
pub use error::ScoringError;
pub use model::*;
pub use query::{
    CacheStats, FilterPreset, Period, PeriodPreset, ScoreQuery, Scoreboards, ScoringCatalog, SignalFilter,
};
