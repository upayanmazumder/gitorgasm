//! Balanced event scoring shared by contributor, repository, and team views.

use crate::ScoringError;
use crate::model::{ContributorScore, RepositoryScore, Ruleset, Signal, TeamMembership, TeamScore};

mod aggregation;
mod points;
mod teams;

pub(crate) use aggregation::scoreboards;

/// Contributor leaderboard for all supplied signals.
///
/// # Errors
/// Returns an error for invalid rules, timestamps, or overflowing totals.
pub fn contributor_leaderboard(signals: &[Signal], rules: &Ruleset) -> Result<Vec<ContributorScore>, ScoringError> {
    Ok(scoreboards(signals.iter(), rules, &[])?.contributors)
}

/// Repository leaderboard using the same scored events as contributors.
///
/// # Errors
/// Returns an error for invalid rules, timestamps, or overflowing totals.
pub fn repository_leaderboard(signals: &[Signal], rules: &Ruleset) -> Result<Vec<RepositoryScore>, ScoringError> {
    Ok(scoreboards(signals.iter(), rules, &[])?.repositories)
}

/// Team leaderboard; team repository restrictions apply after per-user scoring.
///
/// # Errors
/// Returns an error for invalid rules, timestamps, or overflowing totals.
pub fn team_leaderboard(
    signals: &[Signal],
    rules: &Ruleset,
    memberships: &[TeamMembership],
) -> Result<Vec<TeamScore>, ScoringError> {
    Ok(scoreboards(signals.iter(), rules, memberships)?.teams)
}
