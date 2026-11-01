//! Contribution events and their scoring metadata.

use chrono::{DateTime, Timelike, Utc};
use serde::{Deserialize, Serialize};

use crate::ScoringError;

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

impl Signal {
    pub(crate) fn timestamp(&self) -> Result<DateTime<Utc>, ScoringError> {
        DateTime::parse_from_rfc3339(&self.event_timestamp)
            .ok()
            .filter(|time| time.nanosecond() < 1_000_000_000)
            .map(|time| time.to_utc())
            .ok_or_else(|| ScoringError::InvalidTimestamp(self.event_timestamp.clone()))
    }
}
