use serde::{Deserialize, Serialize};

use crate::model::{Signal, SignalType};

/// A built-in group of contribution types.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum FilterPreset {
    /// All types.
    #[default]
    All,
    /// Commits and pull requests.
    Code,
    /// Reviews, review comments, and conversation comments.
    Reviews,
    /// Opened and closed issues.
    Issues,
}

impl FilterPreset {
    fn includes(self, kind: SignalType) -> bool {
        match self {
            Self::All => true,
            Self::Code => matches!(
                kind,
                SignalType::Commit | SignalType::PrOpen | SignalType::PrMerge | SignalType::PrCloseNoMerge
            ),
            Self::Reviews => matches!(
                kind,
                SignalType::Review | SignalType::ReviewComment | SignalType::Comment
            ),
            Self::Issues => matches!(kind, SignalType::IssueOpen | SignalType::IssueClose),
        }
    }
}

/// Filters combine with AND. An empty list accepts all values for that field.
#[derive(Debug, Clone, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", default, deny_unknown_fields)]
pub struct SignalFilter {
    /// Built-in type group.
    pub preset: FilterPreset,
    /// Accepted GitHub user IDs.
    pub users: Vec<i64>,
    /// Accepted repository names. Matches are case-sensitive.
    pub repositories: Vec<String>,
    /// Accepted contribution types within the preset.
    pub kinds: Vec<SignalType>,
    /// Exclude signals whose bot flag is true.
    pub exclude_bots: bool,
    /// ASCII case-insensitive substring of a user login or repository name.
    pub search: Option<String>,
}

fn contains_ascii_fold(haystack: &str, needle: &str) -> bool {
    needle.is_empty()
        || haystack
            .as_bytes()
            .windows(needle.len())
            .any(|window| window.eq_ignore_ascii_case(needle.as_bytes()))
}

impl SignalFilter {
    pub(super) fn includes(&self, signal: &Signal) -> bool {
        self.preset.includes(signal.kind)
            && (self.users.is_empty() || self.users.contains(&signal.user_id))
            && (self.repositories.is_empty() || self.repositories.iter().any(|repo| repo == &signal.repo))
            && (self.kinds.is_empty() || self.kinds.contains(&signal.kind))
            && (!self.exclude_bots || signal.metadata.is_bot != Some(true))
            && self.search.as_ref().is_none_or(|search| {
                let term = search.trim();
                contains_ascii_fold(&signal.user_login, term) || contains_ascii_fold(&signal.repo, term)
            })
    }
}
