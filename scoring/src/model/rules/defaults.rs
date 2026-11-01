use super::{
    BasePointRule, DailyQuota, DiminishingReturns, ReviewStateWeight, Ruleset, ZeroPointCondition, ZeroPointKind,
};
use crate::model::SignalType;

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
