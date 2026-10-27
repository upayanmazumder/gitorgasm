//! Balanced event scoring shared by contributor, repository, and team views.

use std::collections::{HashMap, HashSet};

use chrono::{DateTime, Datelike, NaiveDate, Timelike};

use crate::error::ScoringError;
use crate::model::{
    ContributorScore, MultiplierKind, RepositoryScore, Ruleset, Signal, SignalType, TeamMembership, TeamScore, Totals,
    ZeroPointKind,
};
use crate::query::Scoreboards;

const N: usize = SignalType::COUNT;

struct Compiled<'a> {
    rules: &'a Ruleset,
    base: [f64; N],
    quota: [Option<u32>; N],
    zero: [bool; 5],
}

impl<'a> Compiled<'a> {
    fn new(rules: &'a Ruleset) -> Result<Self, ScoringError> {
        let d = &rules.diminishing_returns;
        let numbers = rules
            .base_points
            .iter()
            .map(|r| ("basePoints.points", r.points))
            .chain(rules.multipliers.iter().map(|r| ("multipliers.factor", r.factor)))
            .chain(
                rules
                    .review_state_weights
                    .iter()
                    .map(|r| ("reviewStateWeights.weight", r.weight)),
            )
            .chain([
                ("diminishingReturns.decayFactor", d.decay_factor),
                ("diminishingReturns.floorFraction", d.floor_fraction),
                ("spamPenalty", rules.spam_penalty),
                ("prClosedNoMergePenalty", rules.pr_closed_no_merge_penalty),
            ]);
        for (field, value) in numbers {
            if !value.is_finite() {
                return Err(ScoringError::NonFiniteRule(field));
            }
            if value < 0.0 && field != "spamPenalty" && field != "prClosedNoMergePenalty" {
                return Err(ScoringError::InvalidRule(field));
            }
        }
        if d.decay_factor > 1.0 || d.floor_fraction > 1.0 {
            return Err(ScoringError::InvalidRule("diminishingReturns"));
        }
        let mut base = [0.0; N];
        for r in &rules.base_points {
            base[r.kind.index()] = r.points;
        }
        let mut quota = [None; N];
        for r in &rules.daily_quotas {
            quota[r.kind.index()] = Some(r.limit);
        }
        let mut zero = [false; 5];
        for condition in &rules.zero_point_conditions {
            zero[condition.kind as usize] = condition.enabled;
        }
        Ok(Self {
            rules,
            base,
            quota,
            zero,
        })
    }

    fn excluded(&self, s: &Signal) -> bool {
        let m = &s.metadata;
        (self.zero[ZeroPointKind::BotActivity as usize] && m.is_bot == Some(true))
            || (self.zero[ZeroPointKind::SelfReview as usize]
                && s.kind == SignalType::Review
                && m.is_self_review == Some(true))
            || (self.zero[ZeroPointKind::SelfMerge as usize]
                && s.kind == SignalType::PrMerge
                && m.is_self_merge == Some(true))
            || (self.zero[ZeroPointKind::IssueClosedNoPr as usize]
                && s.kind == SignalType::IssueClose
                && m.has_linked_pr != Some(true))
            || (self.zero[ZeroPointKind::PrClosedNoMerge as usize] && s.kind == SignalType::PrCloseNoMerge)
    }

    fn points(&self, s: &Signal, state: &mut State, day: NaiveDate, week: (i32, u32)) -> (f64, f64) {
        if self.excluded(s) {
            return (0.0, 0.0);
        }
        match s.kind {
            SignalType::Spam => return (0.0, self.rules.spam_penalty),
            SignalType::PrCloseNoMerge => return (0.0, self.rules.pr_closed_no_merge_penalty),
            _ => {}
        }
        let kind = s.kind.index();
        if self.base[kind] == 0.0 {
            return (0.0, 0.0);
        }
        if state.day != Some(day) {
            state.day = Some(day);
            state.daily = [0; N];
            state.first = [false; N];
        }
        if state.week != Some(week) {
            state.week = Some(week);
            state.weekly = [0; N];
        }
        state.daily[kind] = state.daily[kind].saturating_add(1);
        if self.quota[kind].is_some_and(|limit| state.daily[kind] > limit) {
            return (0.0, 0.0);
        }
        state.weekly[kind] = state.weekly[kind].saturating_add(1);
        let d = &self.rules.diminishing_returns;
        let excess = state.weekly[kind].saturating_sub(d.weekly_threshold);
        let fraction = d.floor_fraction.max(1.0 - f64::from(excess) * d.decay_factor);
        let mut points = self.base[kind] * fraction;
        for multiplier in &self.rules.multipliers {
            if !multiplier.applies_to.contains(&s.kind) {
                continue;
            }
            let applies = match multiplier.kind {
                MultiplierKind::FirstActivity => !state.first[kind],
                MultiplierKind::MergedPrCommit => {
                    s.kind == SignalType::Commit && s.metadata.is_in_merged_pr == Some(true)
                }
                MultiplierKind::PrLinkedToIssue => {
                    s.kind == SignalType::PrMerge && s.metadata.has_linked_issue == Some(true)
                }
            };
            if applies {
                points *= multiplier.factor;
            }
        }
        state.first[kind] = true;
        if s.kind == SignalType::Review
            && let Some(weight) = self
                .rules
                .review_state_weights
                .iter()
                .find(|w| w.state == s.metadata.state.as_deref().unwrap_or("commented"))
        {
            points *= weight.weight;
        }
        (points, 0.0)
    }
}

#[derive(Default)]
struct State {
    day: Option<NaiveDate>,
    week: Option<(i32, u32)>,
    daily: [u32; N],
    weekly: [u32; N],
    first: [bool; N],
}

impl Totals {
    fn record(&mut self, s: &Signal, points: f64, penalty: f64) {
        self.breakdown.penalties += penalty;
        let (score, count) = match s.kind {
            SignalType::Commit => (&mut self.breakdown.commits, &mut self.counts.commits),
            SignalType::PrOpen | SignalType::PrMerge | SignalType::PrCloseNoMerge => {
                (&mut self.breakdown.pull_requests, &mut self.counts.pull_requests)
            }
            SignalType::Review => (&mut self.breakdown.reviews, &mut self.counts.reviews),
            SignalType::ReviewComment => (&mut self.breakdown.review_comments, &mut self.counts.review_comments),
            SignalType::IssueOpen | SignalType::IssueClose => (&mut self.breakdown.issues, &mut self.counts.issues),
            SignalType::Comment => (&mut self.breakdown.comments, &mut self.counts.comments),
            SignalType::Spam => return,
        };
        *score += points;
        *count = count.saturating_add(1);
        if s.kind == SignalType::Commit {
            self.additions = self.additions.saturating_add(s.metadata.additions.unwrap_or(0));
            self.deletions = self.deletions.saturating_add(s.metadata.deletions.unwrap_or(0));
        }
    }

    fn finish(&mut self) -> Result<(), ScoringError> {
        let b = &self.breakdown;
        self.score =
            (b.commits + b.pull_requests + b.reviews + b.review_comments + b.issues + b.comments + b.penalties)
                .max(0.0);
        if [
            self.score,
            b.commits,
            b.pull_requests,
            b.reviews,
            b.review_comments,
            b.issues,
            b.comments,
            b.penalties,
        ]
        .iter()
        .all(|value| value.is_finite())
        {
            Ok(())
        } else {
            Err(ScoringError::NonFiniteTotal)
        }
    }
}

fn rank<T>(rows: &mut [T], score: impl Fn(&T) -> f64, set_rank: impl Fn(&mut T, u32)) {
    rows.sort_by(|a, b| score(b).total_cmp(&score(a)));
    for (i, row) in rows.iter_mut().enumerate() {
        set_rank(row, u32::try_from(i + 1).unwrap_or(u32::MAX));
    }
}

struct TeamEntry {
    row: TeamScore,
    members: HashSet<i64>,
    repositories: HashSet<String>,
}

fn build_teams(memberships: &[TeamMembership]) -> (Vec<TeamEntry>, HashMap<i64, Vec<usize>>) {
    let mut teams: Vec<TeamEntry> = Vec::new();
    let mut team_index: HashMap<&str, usize> = HashMap::new();
    let mut member_teams: HashMap<i64, Vec<usize>> = HashMap::new();
    for membership in memberships {
        let index = *team_index.entry(membership.team_slug.as_str()).or_insert_with(|| {
            let index = teams.len();
            teams.push(TeamEntry {
                row: TeamScore {
                    team_slug: membership.team_slug.clone(),
                    team_name: membership.team_name.clone(),
                    member_count: 0,
                    totals: Totals::default(),
                    rank: 0,
                },
                members: HashSet::new(),
                repositories: HashSet::new(),
            });
            index
        });
        let team = &mut teams[index];
        if team.members.insert(membership.user_id) {
            member_teams.entry(membership.user_id).or_default().push(index);
        }
        if let Some(names) = &membership.repository_names {
            team.repositories.extend(names.iter().cloned());
        }
    }
    for team in &mut teams {
        team.row.member_count = u32::try_from(team.members.len()).unwrap_or(u32::MAX);
    }
    (teams, member_teams)
}

/// Score one snapshot in event time order. Every view receives the same points for each event.
///
/// # Errors
/// Returns an error for invalid rules, timestamps, or overflowing totals.
pub fn scoreboards(
    signals: &[&Signal],
    rules: &Ruleset,
    memberships: &[TeamMembership],
) -> Result<Scoreboards, ScoringError> {
    let compiled = Compiled::new(rules)?;
    let mut ordered = Vec::with_capacity(signals.len());
    for (index, signal) in signals.iter().enumerate() {
        let time = DateTime::parse_from_rfc3339(&signal.event_timestamp)
            .ok()
            .filter(|ts| ts.nanosecond() < 1_000_000_000)
            .ok_or_else(|| ScoringError::InvalidTimestamp(signal.event_timestamp.clone()))?
            .to_utc();
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
