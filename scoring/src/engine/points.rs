use chrono::NaiveDate;

use crate::ScoringError;
use crate::model::{MultiplierKind, Ruleset, Signal, SignalType, ZeroPointKind};

const N: usize = SignalType::COUNT;

pub(super) struct Compiled<'a> {
    rules: &'a Ruleset,
    base: [f64; N],
    quota: [Option<u32>; N],
    zero: [bool; 5],
}

impl<'a> Compiled<'a> {
    pub(super) fn new(rules: &'a Ruleset) -> Result<Self, ScoringError> {
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

    pub(super) fn points(&self, s: &Signal, state: &mut State, day: NaiveDate, week: (i32, u32)) -> (f64, f64) {
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
pub(super) struct State {
    day: Option<NaiveDate>,
    week: Option<(i32, u32)>,
    daily: [u32; N],
    weekly: [u32; N],
    first: [bool; N],
}
