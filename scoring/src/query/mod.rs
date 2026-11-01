//! Scoring options and organization query results.

use serde::{Deserialize, Serialize};

use crate::ScoringError;
use crate::engine::scoreboards;
use crate::model::{ContributorScore, RepositoryScore, Ruleset, TeamScore};

mod catalog;
mod filter;
mod period;

pub use catalog::{CacheStats, ScoringCatalog};
pub use filter::{FilterPreset, SignalFilter};
pub use period::{Period, PeriodPreset};

/// One set of scoring options. Omitted fields use the balanced defaults.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", default, deny_unknown_fields)]
pub struct ScoreQuery {
    /// Selected period.
    pub period: Period,
    /// Selected signals.
    pub filter: SignalFilter,
    /// Custom points, quotas, multipliers, and penalties.
    pub rules: Ruleset,
}

impl Default for ScoreQuery {
    fn default() -> Self {
        Self {
            period: Period::All,
            filter: SignalFilter::default(),
            rules: Ruleset::balanced_default(),
        }
    }
}
impl ScoreQuery {
    /// Validate time bounds and scoring rules without scoring any events.
    ///
    /// # Errors
    /// Returns an error for an invalid period or ruleset.
    pub fn validate(&self) -> Result<(), ScoringError> {
        self.period.bounds()?;
        scoreboards(std::iter::empty(), &self.rules, &[])?;
        Ok(())
    }
}

/// Contributor, repository, and team rankings for one organization query.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Scoreboards {
    /// Ranked contributors.
    pub contributors: Vec<ContributorScore>,
    /// Ranked repositories.
    pub repositories: Vec<RepositoryScore>,
    /// Ranked teams. Teams without selected signals retain a zero-point row.
    pub teams: Vec<TeamScore>,
}
