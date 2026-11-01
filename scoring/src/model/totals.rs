use serde::{Deserialize, Serialize};

use super::{Signal, SignalType};
use crate::ScoringError;

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

impl Totals {
    pub(crate) fn record(&mut self, s: &Signal, points: f64, penalty: f64) {
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

    pub(crate) fn finish(&mut self) -> Result<(), ScoringError> {
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
