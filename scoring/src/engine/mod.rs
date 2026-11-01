//! Balanced event scoring shared by contributor, repository, and team views.

use std::collections::HashMap;

use chrono::Datelike;

use crate::ScoringError;
use crate::model::{ContributorScore, RepositoryScore, Ruleset, Signal, TeamMembership, TeamScore, Totals};
use crate::query::Scoreboards;

mod points;
mod teams;

use points::{Compiled, State};
use teams::build_teams;

fn rank<T>(rows: &mut [T], score: impl Fn(&T) -> f64, set_rank: impl Fn(&mut T, u32)) {
    rows.sort_by(|a, b| score(b).total_cmp(&score(a)));
    for (i, row) in rows.iter_mut().enumerate() {
        set_rank(row, u32::try_from(i + 1).unwrap_or(u32::MAX));
    }
}

/// Score one snapshot in event time order. Every view receives the same points for each event.
pub(crate) fn scoreboards(
    signals: &[&Signal],
    rules: &Ruleset,
    memberships: &[TeamMembership],
) -> Result<Scoreboards, ScoringError> {
    let compiled = Compiled::new(rules)?;
    let mut ordered = Vec::with_capacity(signals.len());
    for (index, signal) in signals.iter().enumerate() {
        let time = signal.timestamp()?;
        ordered.push((time, index));
    }
    ordered.sort_by_key(|(time, _)| *time);

    let (mut teams, member_teams) = build_teams(memberships);

    let mut contributors = Vec::<ContributorScore>::new();
    let mut contributor_index = HashMap::<i64, usize>::new();
    let mut states = HashMap::<i64, State>::new();
    let mut repositories = Vec::<RepositoryScore>::new();
    let mut repository_index = HashMap::<&str, usize>::new();
    for (time, index) in ordered {
        let s = signals[index];
        let day = time.date_naive();
        let week = day.iso_week();
        let (points, penalty) =
            compiled.points(s, states.entry(s.user_id).or_default(), day, (week.year(), week.week()));
        let user = *contributor_index.entry(s.user_id).or_insert_with(|| {
            let index = contributors.len();
            contributors.push(ContributorScore {
                user_id: s.user_id,
                user_login: s.user_login.clone(),
                totals: Totals::default(),
                rank: 0,
            });
            index
        });
        if contributors[user].user_login != s.user_login {
            contributors[user].user_login.clone_from(&s.user_login);
        }
        contributors[user].totals.record(s, points, penalty);
        let repo = *repository_index.entry(s.repo.as_str()).or_insert_with(|| {
            let index = repositories.len();
            repositories.push(RepositoryScore {
                repo_name: s.repo.clone(),
                totals: Totals::default(),
                rank: 0,
            });
            index
        });
        repositories[repo].totals.record(s, points, penalty);
        if let Some(indices) = member_teams.get(&s.user_id) {
            for &index in indices {
                let team = &mut teams[index];
                if team.repositories.is_empty() || team.repositories.contains(&s.repo) {
                    team.row.totals.record(s, points, penalty);
                }
            }
        }
    }
    for row in &mut contributors {
        row.totals.finish()?;
    }
    for row in &mut repositories {
        row.totals.finish()?;
    }
    let mut teams: Vec<TeamScore> = teams.into_iter().map(|team| team.row).collect();
    for row in &mut teams {
        row.totals.finish()?;
    }
    rank(&mut contributors, |row| row.totals.score, |row, rank| row.rank = rank);
    rank(&mut repositories, |row| row.totals.score, |row, rank| row.rank = rank);
    rank(&mut teams, |row| row.totals.score, |row, rank| row.rank = rank);
    Ok(Scoreboards {
        contributors,
        repositories,
        teams,
    })
}

/// Contributor leaderboard for all supplied signals.
///
/// # Errors
/// Returns an error for invalid rules, timestamps, or overflowing totals.
pub fn contributor_leaderboard(signals: &[Signal], rules: &Ruleset) -> Result<Vec<ContributorScore>, ScoringError> {
    Ok(scoreboards(&signals.iter().collect::<Vec<_>>(), rules, &[])?.contributors)
}

/// Repository leaderboard using the same scored events as contributors.
///
/// # Errors
/// Returns an error for invalid rules, timestamps, or overflowing totals.
pub fn repository_leaderboard(signals: &[Signal], rules: &Ruleset) -> Result<Vec<RepositoryScore>, ScoringError> {
    Ok(scoreboards(&signals.iter().collect::<Vec<_>>(), rules, &[])?.repositories)
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
    Ok(scoreboards(&signals.iter().collect::<Vec<_>>(), rules, memberships)?.teams)
}
