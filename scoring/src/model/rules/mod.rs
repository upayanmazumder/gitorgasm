use serde::{Deserialize, Serialize};

use super::SignalType;

mod defaults;

/// Points for one signal type.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct BasePointRule {
    /// Signal type.
    #[serde(rename = "type")]
    pub kind: SignalType,
    /// Base points.
    pub points: f64,
}

/// Multiplier kinds.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum MultiplierKind {
    /// Commit that is part of a merged pull request.
    MergedPrCommit,
    /// Pull request that closes an issue.
    PrLinkedToIssue,
    /// First signal of a type per user per day.
    FirstActivity,
}

/// A multiplier and the signal types it applies to.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct MultiplierRule {
    /// Kind. Rules of the same kind are combined multiplicatively.
    pub kind: MultiplierKind,
    /// Factor.
    pub factor: f64,
    /// Signal types it applies to.
    pub applies_to: Vec<SignalType>,
}

/// Weight for a review state.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct ReviewStateWeight {
    /// Review state, for example `approved`.
    pub state: String,
    /// Weight.
    pub weight: f64,
}

/// Maximum scored signals of one type per user per day.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct DailyQuota {
    /// Signal type.
    #[serde(rename = "type")]
    pub kind: SignalType,
    /// Limit.
    pub limit: u32,
}

/// Weekly diminishing returns.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DiminishingReturns {
    /// Occurrences per UTC week that score full points.
    pub weekly_threshold: u32,
    /// Fraction removed per occurrence above the threshold.
    pub decay_factor: f64,
    /// Lowest fraction of base points.
    pub floor_fraction: f64,
}

/// Conditions that make a signal score zero.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ZeroPointKind {
    /// Review of one's own pull request.
    SelfReview,
    /// Merge of one's own pull request.
    SelfMerge,
    /// Any bot signal.
    BotActivity,
    /// Issue closed without a linked pull request.
    IssueClosedNoPr,
    /// Pull request closed without merge.
    PrClosedNoMerge,
}

/// A zero-point condition and whether it is enabled.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct ZeroPointCondition {
    /// Condition.
    pub kind: ZeroPointKind,
    /// Enabled.
    pub enabled: bool,
}

/// A complete scoring preset.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct Ruleset {
    /// Base points per type. Types without a rule score 0.
    pub base_points: Vec<BasePointRule>,
    /// Multipliers.
    pub multipliers: Vec<MultiplierRule>,
    /// Review state weights. Unknown states weigh 1.
    pub review_state_weights: Vec<ReviewStateWeight>,
    /// Daily quotas.
    pub daily_quotas: Vec<DailyQuota>,
    /// Weekly diminishing returns.
    pub diminishing_returns: DiminishingReturns,
    /// Zero-point conditions.
    pub zero_point_conditions: Vec<ZeroPointCondition>,
    /// Penalty per spam signal (negative).
    pub spam_penalty: f64,
    /// Penalty per pull request closed without merge (negative).
    pub pr_closed_no_merge_penalty: f64,
}
