//! Legacy-compatible scoring (S1–S9); change scoring quirks only behind a ruleset version.

use std::collections::{HashMap, HashSet};
use std::hash::Hash;

use chrono::{DateTime, Datelike, Days, Timelike};

use crate::error::ScoringError;
use crate::model::{
    ContributorScore, MultiplierKind, RepositoryScore, Ruleset, Signal, SignalType, TeamMembership, TeamScore, Totals,
    ZeroPointKind,
};

const N: usize = SignalType::COUNT;

/// A multiplier resolved to the first rule of its kind, as legacy `Array.find` does.
#[derive(Clone, Copy)]
struct Multiplier {
    factor: f64,
    applies: [bool; N],
}

struct Compiled<'r> {
    rules: &'r Ruleset,
    base: [f64; N],
    quota: [Option<u32>; N],
    first_activity: Option<Multiplier>,
    merged_pr_commit: Option<Multiplier>,
    pr_linked_to_issue: Option<Multiplier>,
}

impl<'r> Compiled<'r> {
    fn new(rules: &'r Ruleset, with_first_activity: bool) -> Result<Self, ScoringError> {
        check_finite(rules)?;
        let mut base = [0.0; N];
        let mut base_set = [false; N];
        for r in &rules.base_points {
            if !base_set[r.kind.index()] {
                base_set[r.kind.index()] = true;
                base[r.kind.index()] = r.points;
            }
        }
        let mut quota = [None; N];
        for q in &rules.daily_quotas {
            quota[q.kind.index()].get_or_insert(q.limit);
        }
        let find = |kind| {
            rules.multipliers.iter().find(|m| m.kind == kind).map(|m| {
                let mut applies = [false; N];
                for t in &m.applies_to {
                    applies[t.index()] = true;
                }
                Multiplier {
                    factor: m.factor,
                    applies,
                }
            })
        };
        Ok(Self {
            rules,
            base,
            quota,
            // Repository scoring removes every first_activity rule.
            first_activity: if with_first_activity {
                find(MultiplierKind::FirstActivity)
            } else {
                None
            },
            merged_pr_commit: find(MultiplierKind::MergedPrCommit),
            pr_linked_to_issue: find(MultiplierKind::PrLinkedToIssue),
        })
    }

    fn zero_point(&self, s: &Signal) -> bool {
        let m = &s.metadata;
        self.rules
            .zero_point_conditions
            .iter()
            .filter(|c| c.enabled)
            .any(|c| match c.kind {
                ZeroPointKind::SelfReview => s.kind == SignalType::Review && m.is_self_review == Some(true),
                ZeroPointKind::SelfMerge => s.kind == SignalType::PrMerge && m.is_self_merge == Some(true),
                ZeroPointKind::BotActivity => m.is_bot == Some(true),
                ZeroPointKind::PrClosedNoMerge => s.kind == SignalType::PrCloseNoMerge,
                ZeroPointKind::IssueClosedNoPr => s.kind == SignalType::IssueClose && m.has_linked_pr != Some(true),
            })
    }

    fn diminishing(&self, occurrence: u32, base: f64) -> f64 {
        let d = &self.rules.diminishing_returns;
        if occurrence <= d.weekly_threshold {
            return base;
        }
        let factor = 1.0 - f64::from(occurrence - d.weekly_threshold) * d.decay_factor;
        (base * d.floor_fraction.max(factor)).max(0.0)
    }

    fn score<'s>(&self, s: &'s Signal, ctx: &mut Context<'s>, skip_quota: bool) -> (f64, f64) {
        let t = s.kind.index();
        if self.zero_point(s) {
            return (0.0, 0.0);
        }
        match s.kind {
            SignalType::Spam => return (0.0, self.rules.spam_penalty),
            SignalType::PrCloseNoMerge => return (0.0, self.rules.pr_closed_no_merge_penalty),
            _ => {}
        }
        let base = self.base[t];
        if base == 0.0 {
            return (0.0, 0.0);
        }

        let effective = if skip_quota {
            base
        } else {
            // S5: the day key is the raw string prefix, which keeps the author's UTC offset.
            let day = s.event_timestamp.get(..10).unwrap_or(&s.event_timestamp);
            if ctx.current_day != Some(day) {
                ctx.current_day = Some(day);
                ctx.daily = [0; N];
                ctx.first_seen = [false; N];
            }
            // S4: both counters grow before the quota test.
            ctx.daily[t] += 1;
            ctx.weekly[t] += 1;
            match self.quota[t] {
                Some(limit) if ctx.daily[t] > limit => 0.0,
                _ => self.diminishing(ctx.weekly[t] - 1, base),
            }
        };

        let mut multiplier = 1.0;
        if let Some(m) = self.first_activity
            && m.applies[t]
            && !ctx.first_seen[t]
        {
            ctx.first_seen[t] = true;
            multiplier *= m.factor;
        }
        if let Some(m) = self.merged_pr_commit
            && m.applies[t]
            && s.metadata.is_in_merged_pr == Some(true)
        {
            multiplier *= m.factor;
        }
        if let Some(m) = self.pr_linked_to_issue
            && m.applies[t]
            && s.metadata.has_linked_issue == Some(true)
        {
            multiplier *= m.factor;
        }
        if s.kind == SignalType::Review {
            let state = s.metadata.state.as_deref().unwrap_or("commented");
            let weight = self
                .rules
                .review_state_weights
                .iter()
                .find(|w| w.state == state)
                .map_or(1.0, |w| w.weight);
            multiplier *= weight;
        }
        (effective * multiplier, 0.0)
    }
}

fn check_finite(rules: &Ruleset) -> Result<(), ScoringError> {
    let d = &rules.diminishing_returns;
    let fixed = [
        ("diminishingReturns.decayFactor", d.decay_factor),
        ("diminishingReturns.floorFraction", d.floor_fraction),
        ("spamPenalty", rules.spam_penalty),
        ("prClosedNoMergePenalty", rules.pr_closed_no_merge_penalty),
    ];
    let bad = rules
        .base_points
        .iter()
        .map(|r| ("basePoints.points", r.points))
        .chain(rules.multipliers.iter().map(|m| ("multipliers.factor", m.factor)))
        .chain(
            rules
                .review_state_weights
                .iter()
                .map(|w| ("reviewStateWeights.weight", w.weight)),
        )
        .chain(fixed)
        .find(|(_, v)| !v.is_finite());
    match bad {
        Some((field, _)) => Err(ScoringError::NonFiniteRule(field)),
        None => Ok(()),
    }
}

struct Context<'s> {
    first_seen: [bool; N],
    weekly: [u32; N],
    daily: [u32; N],
    current_day: Option<&'s str>,
}

impl Context<'_> {
    fn new() -> Self {
        Self {
            first_seen: [false; N],
            weekly: [0; N],
            daily: [0; N],
            current_day: None,
        }
    }
}

struct Timed<'s> {
    signal: &'s Signal,
    millis: i64,
    /// Days since the epoch of the Monday (UTC) that starts the signal's week.
    week: i32,
}

fn timed<'s>(signals: &[&'s Signal]) -> Result<Vec<Timed<'s>>, ScoringError> {
    let mut out = Vec::with_capacity(signals.len());
    for &signal in signals {
        let ts = DateTime::parse_from_rfc3339(&signal.event_timestamp)
            .ok()
            // chrono accepts leap seconds; legacy `new Date()` rejects them.
            .filter(|ts| ts.nanosecond() < 1_000_000_000)
            .ok_or_else(|| ScoringError::InvalidTimestamp(signal.event_timestamp.clone()))?;
        let utc = ts.to_utc();
        let date = utc.date_naive();
        let monday = date
            .checked_sub_days(Days::new(u64::from(date.weekday().num_days_from_monday())))
            .unwrap_or(date);
        out.push(Timed {
            signal,
            millis: utc.timestamp_millis(),
            week: monday.num_days_from_ce(),
        });
    }
    // Stable sort, as legacy Array.prototype.sort: equal instants keep input order.
    out.sort_by_key(|t| t.millis);
    Ok(out)
}

impl Totals {
    fn add_signal(&mut self, s: &Signal, points: f64, penalty: f64, running: &mut (f64, f64)) {
        running.0 += points;
        running.1 += penalty;
        let (b, c) = (&mut self.breakdown, &mut self.counts);
        let (score, count) = match s.kind {
            SignalType::Commit => (&mut b.commits, &mut c.commits),
            SignalType::PrOpen | SignalType::PrMerge | SignalType::PrCloseNoMerge => {
                (&mut b.pull_requests, &mut c.pull_requests)
            }
            SignalType::Review => (&mut b.reviews, &mut c.reviews),
            SignalType::ReviewComment => (&mut b.review_comments, &mut c.review_comments),
            SignalType::IssueOpen | SignalType::IssueClose => (&mut b.issues, &mut c.issues),
            SignalType::Comment => (&mut b.comments, &mut c.comments),
            SignalType::Spam => return,
        };
        *score += points;
        *count += 1;
        if s.kind == SignalType::Commit {
            // Line counts are untrusted input. Saturate instead of overflowing. Legacy summed f64 values.
            self.additions = self.additions.saturating_add(s.metadata.additions.unwrap_or(0));
            self.deletions = self.deletions.saturating_add(s.metadata.deletions.unwrap_or(0));
        }
    }

    fn finish(&mut self, running: (f64, f64)) -> Result<(), ScoringError> {
        self.breakdown.penalties = running.1;
        self.score = 0f64.max(running.0 + running.1);
        self.check_finite()
    }

    /// Rejects totals that overflowed. `f64::max` maps NaN to 0, so the parts are checked as well.
    fn check_finite(&self) -> Result<(), ScoringError> {
        let b = &self.breakdown;
        let finite = [
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
        .all(|v| v.is_finite());
        if finite {
            Ok(())
        } else {
            Err(ScoringError::NonFiniteTotal)
        }
    }

    fn absorb(&mut self, o: &Totals) {
        let (b, ob) = (&mut self.breakdown, &o.breakdown);
        self.score += o.score;
        b.commits += ob.commits;
        b.pull_requests += ob.pull_requests;
        b.reviews += ob.reviews;
        b.review_comments += ob.review_comments;
        b.issues += ob.issues;
        b.comments += ob.comments;
        b.penalties += ob.penalties;
        let (c, oc) = (&mut self.counts, &o.counts);
        c.commits += oc.commits;
        c.pull_requests += oc.pull_requests;
        c.reviews += oc.reviews;
        c.review_comments += oc.review_comments;
        c.issues += oc.issues;
        c.comments += oc.comments;
        self.additions = self.additions.saturating_add(o.additions);
        self.deletions = self.deletions.saturating_add(o.deletions);
    }
}

/// Groups items by key in first-appearance order, as a JavaScript `Map` iterates.
fn group_by<'a, T, K: Hash + Eq>(items: impl IntoIterator<Item = &'a T>, key: impl Fn(&'a T) -> K) -> Vec<Vec<&'a T>> {
    let mut index = HashMap::new();
    let mut groups: Vec<Vec<&T>> = Vec::new();
    for item in items {
        let i = *index.entry(key(item)).or_insert_with(|| {
            groups.push(Vec::new());
            groups.len() - 1
        });
        groups[i].push(item);
    }
    groups
}

/// Sorts by score descending (stable) and assigns 1-based ranks. S8: ties get distinct ranks.
fn rank<T>(mut rows: Vec<T>, score: impl Fn(&T) -> f64, set_rank: impl Fn(&mut T, u32)) -> Vec<T> {
    // Scores are finite and never -0.0 here, so `total_cmp` orders them as legacy `b - a` does.
    rows.sort_by(|a, b| score(b).total_cmp(&score(a)));
    for (i, row) in rows.iter_mut().enumerate() {
        set_rank(row, u32::try_from(i + 1).unwrap_or(u32::MAX));
    }
    rows
}

/// Per-user totals in first-appearance order (legacy `computeScores`).
fn user_totals<'s>(signals: &[&'s Signal], compiled: &Compiled) -> Result<Vec<(&'s Signal, Totals)>, ScoringError> {
    let mut out = Vec::new();
    for group in group_by(signals.iter().copied(), |s| s.user_id) {
        let mut ctx = Context::new();
        let mut totals = Totals::default();
        let mut running = (0.0, 0.0);
        let mut last_week = None;
        for t in timed(&group)? {
            if last_week != Some(t.week) {
                last_week = Some(t.week);
                ctx.weekly = [0; N];
            }
            let (points, penalty) = compiled.score(t.signal, &mut ctx, false);
            totals.add_signal(t.signal, points, penalty, &mut running);
        }
        totals.finish(running)?;
        out.push((group[0], totals));
    }
    Ok(out)
}

/// Contributor leaderboard (legacy `buildLeaderboard(computeScores(...))`).
///
/// # Errors
/// See [`ScoringError`]: an invalid timestamp, a non-finite ruleset number, or an overflowed total.
pub fn contributor_leaderboard(signals: &[Signal], rules: &Ruleset) -> Result<Vec<ContributorScore>, ScoringError> {
    let compiled = Compiled::new(rules, true)?;
    let refs: Vec<&Signal> = signals.iter().collect();
    let rows = user_totals(&refs, &compiled)?
        .into_iter()
        .map(|(first, totals)| ContributorScore {
            user_id: first.user_id,
            user_login: first.user_login.clone(),
            totals,
            rank: 0,
        })
        .collect();
    Ok(rank(rows, |r| r.totals.score, |r, n| r.rank = n))
}

/// Repository leaderboard: unlike contributors, no quota, decay, or first-activity multiplier.
///
/// # Errors
/// See [`ScoringError`]: an invalid timestamp, a non-finite ruleset number, or an overflowed total.
pub fn repository_leaderboard(signals: &[Signal], rules: &Ruleset) -> Result<Vec<RepositoryScore>, ScoringError> {
    let compiled = Compiled::new(rules, false)?;
    let mut rows = Vec::new();
    for group in group_by(signals, |s| s.repo.as_str()) {
        let mut ctx = Context::new();
        let mut totals = Totals::default();
        let mut running = (0.0, 0.0);
        for t in timed(&group)? {
            let (points, penalty) = compiled.score(t.signal, &mut ctx, true);
            totals.add_signal(t.signal, points, penalty, &mut running);
        }
        totals.finish(running)?;
        rows.push(RepositoryScore {
            repo_name: group[0].repo.clone(),
            totals,
            rank: 0,
        });
    }
    Ok(rank(rows, |r| r.totals.score, |r, n| r.rank = n))
}

/// Team leaderboard: sum per-member scores, filtered to listed repositories when present.
///
/// # Errors
/// See [`ScoringError`]: an invalid timestamp, a non-finite ruleset number, or an overflowed total.
pub fn team_leaderboard(
    signals: &[Signal],
    rules: &Ruleset,
    memberships: &[TeamMembership],
) -> Result<Vec<TeamScore>, ScoringError> {
    let compiled = Compiled::new(rules, true)?;
    let mut rows = Vec::new();
    for team in group_by(memberships, |m| m.team_slug.as_str()) {
        let members: HashSet<i64> = team.iter().map(|m| m.user_id).collect();
        let repos: HashSet<&str> = team
            .iter()
            .flat_map(|m| m.repository_names.iter().flatten())
            .map(String::as_str)
            .collect();
        let selected: Vec<&Signal> = signals
            .iter()
            .filter(|s| members.contains(&s.user_id) && (repos.is_empty() || repos.contains(s.repo.as_str())))
            .collect();
        let mut totals = Totals::default();
        for (_, user) in user_totals(&selected, &compiled)? {
            totals.absorb(&user);
        }
        totals.check_finite()?;
        rows.push(TeamScore {
            team_slug: team[0].team_slug.clone(),
            team_name: team[0].team_name.clone(),
            member_count: u32::try_from(members.len()).unwrap_or(u32::MAX),
            totals,
            rank: 0,
        });
    }
    Ok(rank(rows, |r| r.totals.score, |r, n| r.rank = n))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::model::Metadata;

    fn signal(kind: SignalType, ts: &str) -> Signal {
        Signal {
            user_id: 1,
            user_login: "u".into(),
            kind,
            repo: "r".into(),
            event_timestamp: ts.into(),
            metadata: Metadata::default(),
        }
    }

    #[test]
    fn invalid_timestamp_is_an_error() {
        let err = contributor_leaderboard(&[signal(SignalType::Commit, "yesterday")], &Ruleset::legacy_default());
        assert_eq!(err, Err(ScoringError::InvalidTimestamp("yesterday".into())));
    }

    #[test]
    fn eleventh_weekly_commit_is_diminished() {
        let days = [
            "2026-01-05",
            "2026-01-05",
            "2026-01-05",
            "2026-01-06",
            "2026-01-06",
            "2026-01-06",
            "2026-01-07",
            "2026-01-07",
            "2026-01-07",
            "2026-01-08",
            "2026-01-08",
        ];
        let signals: Vec<_> = days
            .iter()
            .enumerate()
            .map(|(i, d)| signal(SignalType::Commit, &format!("{d}T{i:02}:00:00Z")))
            .collect();
        let rows = contributor_leaderboard(&signals, &Ruleset::legacy_default()).unwrap();
        let expected = 4.0 * 15.0 + 6.0 * 10.0 + 10.0 * (1.0 - 0.11);
        assert!(
            (rows[0].totals.score - expected).abs() < 1e-9,
            "{}",
            rows[0].totals.score
        );
    }
}
