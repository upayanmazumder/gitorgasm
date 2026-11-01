use serde::{Deserialize, Serialize};

use super::Totals;

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
