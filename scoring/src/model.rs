//! Contribution events, configurable scoring rules, and leaderboard rows.

use serde::{Deserialize, Serialize};

/// Kind of contribution event.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum SignalType {
    /// Commit on the default branch.
    Commit,
    /// Pull request opened.
    PrOpen,
    /// Pull request merged.
    PrMerge,
    /// Pull request closed without merge.
    PrCloseNoMerge,
    /// Issue opened.
    IssueOpen,
    /// Issue closed, credited to the actor who closed it.
    IssueClose,
    /// Pull request review.
    Review,
    /// Inline review comment.
    ReviewComment,
    /// Pull request conversation comment.
    Comment,
    /// Explicit spam marker.
    Spam,
}

impl SignalType {
    /// Number of signal types. Per-type counters are arrays of this length.
    pub const COUNT: usize = 10;

    pub(crate) fn index(self) -> usize {
        self as usize
    }
}

/// Signal metadata that scoring reads. Absent flags count as `false`.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", default)]
pub struct Metadata {
    /// Author is a bot.
    pub is_bot: Option<bool>,
    /// Review of the reviewer's own pull request.
    pub is_self_review: Option<bool>,
    /// Pull request merged by its author.
    pub is_self_merge: Option<bool>,
    /// Commit belongs to a merged pull request.
    #[serde(rename = "isInMergedPR")]
    pub is_in_merged_pr: Option<bool>,
    /// Pull request closes an issue.
    pub has_linked_issue: Option<bool>,
    /// Closed issue is referenced by a pull request.
    #[serde(rename = "hasLinkedPR")]
    pub has_linked_pr: Option<bool>,
    /// Review state such as `approved`. Absent means `commented`.
    pub state: Option<String>,
    /// Lines added (commits only).
    pub additions: Option<i64>,
    /// Lines deleted (commits only).
    pub deletions: Option<i64>,
}

/// One contribution event attributed to a user.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Signal {
    /// GitHub user id.
    pub user_id: i64,
    /// GitHub login at ingest time.
    pub user_login: String,
    /// Event kind.
    #[serde(rename = "type")]
    pub kind: SignalType,
    /// Repository name.
    pub repo: String,
    /// RFC 3339 event timestamp. Calendar quotas use UTC days and ISO weeks.
    pub event_timestamp: String,
    /// Scoring flags.
    #[serde(default)]
    pub metadata: Metadata,
}

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

impl Ruleset {
    /// Balanced, activity-based defaults. Low-cost events cannot overwhelm reviews or PRs.
    #[must_use]
    pub fn balanced_default() -> Self {
        use SignalType::{Commit, IssueClose, IssueOpen, PrMerge, PrOpen, Review};
        let base = |kind, points| BasePointRule { kind, points };
        let quota = |kind, limit| DailyQuota { kind, limit };
        Self {
            base_points: vec![
                base(Commit, 1.0),
                base(PrOpen, 4.0),
                base(PrMerge, 4.0),
                base(Review, 3.0),
                base(IssueOpen, 2.0),
                base(IssueClose, 1.0),
            ],
            multipliers: vec![],
            review_state_weights: vec![
                ReviewStateWeight {
                    state: "approved".into(),
                    weight: 1.0,
                },
                ReviewStateWeight {
                    state: "changes_requested".into(),
                    weight: 1.0,
                },
                ReviewStateWeight {
                    state: "commented".into(),
                    weight: 1.0,
                },
            ],
            daily_quotas: vec![
                quota(Commit, 10),
                quota(PrOpen, 10),
                quota(PrMerge, 10),
                quota(Review, 10),
                quota(IssueOpen, 10),
                quota(IssueClose, 10),
            ],
            diminishing_returns: DiminishingReturns {
                weekly_threshold: 20,
                decay_factor: 0.05,
                floor_fraction: 0.25,
            },
            zero_point_conditions: vec![
                ZeroPointCondition {
                    kind: ZeroPointKind::BotActivity,
                    enabled: true,
                },
                ZeroPointCondition {
                    kind: ZeroPointKind::SelfReview,
                    enabled: true,
                },
            ],
            spam_penalty: 0.0,
            pr_closed_no_merge_penalty: 0.0,
        }
    }
}

/// Points per contribution category.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ScoreBreakdown {
    /// Commits.
    pub commits: f64,
    /// Pull request open, merge, and close.
    pub pull_requests: f64,
    /// Reviews.
    pub reviews: f64,
    /// Review comments.
    pub review_comments: f64,
    /// Issue open and close.
    pub issues: f64,
    /// Comments.
    pub comments: f64,
    /// Sum of penalties (negative or zero).
    pub penalties: f64,
}

/// Signal counts per contribution category, including zero-point signals.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ContributionCounts {
    /// Commits.
    pub commits: u32,
    /// Pull request open, merge, and close.
    pub pull_requests: u32,
    /// Reviews.
    pub reviews: u32,
    /// Review comments.
    pub review_comments: u32,
    /// Issue open and close.
    pub issues: u32,
    /// Comments.
    pub comments: u32,
}

/// Totals shared by every leaderboard row.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Totals {
    /// Sum of points and penalties, floored at zero for each leaderboard row.
    pub score: f64,
    /// Points per category.
    pub breakdown: ScoreBreakdown,
    /// Signal counts per category.
    pub counts: ContributionCounts,
    /// Lines added by commits.
    pub additions: i64,
    /// Lines deleted by commits.
    pub deletions: i64,
}

/// Contributor leaderboard row.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ContributorScore {
    /// GitHub user id.
    pub user_id: i64,
    /// Most recently observed login for this user by UTC event time.
    pub user_login: String,
    /// Totals.
    #[serde(flatten)]
    pub totals: Totals,
    /// 1-based rank; ties retain first appearance order.
    pub rank: u32,
}

/// Repository leaderboard row.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RepositoryScore {
    /// Repository name.
    pub repo_name: String,
    /// Totals.
    #[serde(flatten)]
    pub totals: Totals,
    /// 1-based rank.
    pub rank: u32,
}

/// Team membership used for team scoring.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TeamMembership {
    /// Member's GitHub user id.
    pub user_id: i64,
    /// Team slug.
    pub team_slug: String,
    /// Team display name.
    pub team_name: String,
    /// When any membership of a team lists repositories, only signals in those count.
    #[serde(default)]
    pub repository_names: Option<Vec<String>>,
}

/// Team leaderboard row.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TeamScore {
    /// Team slug.
    pub team_slug: String,
    /// Team name.
    pub team_name: String,
    /// Distinct members.
    pub member_count: u32,
    /// Sum of member totals.
    #[serde(flatten)]
    pub totals: Totals,
    /// 1-based rank.
    pub rank: u32,
}
