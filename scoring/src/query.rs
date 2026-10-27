//! Organization-scoped queries and bounded result caching.

use std::collections::{HashMap, VecDeque};
use std::num::NonZeroUsize;

use chrono::{DateTime, Datelike, Days, TimeDelta, Timelike, Utc};
use serde::{Deserialize, Serialize};

use crate::engine::scoreboards;
use crate::{ContributorScore, RepositoryScore, Ruleset, ScoringError, Signal, SignalType, TeamMembership, TeamScore};

/// A UTC period anchored to an explicit instant.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum PeriodPreset {
    /// Since midnight UTC today.
    Today,
    /// Since Monday midnight UTC.
    ThisWeek,
    /// Since the first day of this UTC month.
    ThisMonth,
    /// The previous seven 24-hour days.
    Last7Days,
    /// The previous thirty 24-hour days.
    Last30Days,
}

/// A time interval. Every bounded interval includes its start and excludes its end.
#[derive(Debug, Clone, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(tag = "kind", rename_all = "snake_case", deny_unknown_fields)]
pub enum Period {
    /// Include all signals.
    #[default]
    All,
    /// Resolve a period in UTC using `as_of` as the exclusive end.
    Preset {
        /// Named period.
        preset: PeriodPreset,
        /// Exclusive end. Supplying it explicitly keeps results and cache keys stable.
        as_of: DateTime<Utc>,
    },
    /// An arbitrary UTC interval with an inclusive start and exclusive end.
    Custom {
        /// Inclusive start.
        start: DateTime<Utc>,
        /// Exclusive end.
        end: DateTime<Utc>,
    },
}

type TimeBounds = (DateTime<Utc>, DateTime<Utc>);

impl Period {
    fn bounds(&self) -> Result<Option<TimeBounds>, ScoringError> {
        let (start, end) = match self {
            Self::All => return Ok(None),
            Self::Custom { start, end } => (*start, *end),
            Self::Preset { preset, as_of } => {
                let date = as_of.date_naive();
                let start = match preset {
                    PeriodPreset::Last7Days => as_of.checked_sub_signed(TimeDelta::days(7)),
                    PeriodPreset::Last30Days => as_of.checked_sub_signed(TimeDelta::days(30)),
                    PeriodPreset::Today | PeriodPreset::ThisWeek | PeriodPreset::ThisMonth => {
                        let day = match preset {
                            PeriodPreset::Today => Some(date),
                            PeriodPreset::ThisWeek => {
                                date.checked_sub_days(Days::new(u64::from(date.weekday().num_days_from_monday())))
                            }
                            PeriodPreset::ThisMonth => date.with_day(1),
                            PeriodPreset::Last7Days | PeriodPreset::Last30Days => None,
                        };
                        day.and_then(|d| d.and_hms_opt(0, 0, 0))
                            .map(|d| DateTime::<Utc>::from_naive_utc_and_offset(d, Utc))
                    }
                };
                (start.ok_or(ScoringError::InvalidPeriod)?, *as_of)
            }
        };
        if start >= end {
            return Err(ScoringError::InvalidPeriod);
        }
        Ok(Some((start, end)))
    }
}

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
    fn includes(&self, signal: &Signal) -> bool {
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

/// One set of scoring options. Omitted fields use the balanced defaults.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", default, deny_unknown_fields)]
pub struct ScoreQuery {
    /// Selected period.
    pub period: Period,
    /// Selected signals.
    pub filter: SignalFilter,
    /// Custom points, quotas, multipliers, and penalties.
    pub rules: Ruleset,
}

impl Default for ScoreQuery {
    fn default() -> Self {
        Self {
            period: Period::All,
            filter: SignalFilter::default(),
            rules: Ruleset::balanced_default(),
        }
    }
}
impl ScoreQuery {
    /// Validate time bounds and scoring rules without scoring any events.
    ///
    /// # Errors
    /// Returns an error for an invalid period or ruleset.
    pub fn validate(&self) -> Result<(), ScoringError> {
        self.period.bounds()?;
        scoreboards(&[], &self.rules, &[])?;
        Ok(())
    }
}

/// Contributor, repository, and team rankings for one organization query.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Scoreboards {
    /// Ranked contributors.
    pub contributors: Vec<ContributorScore>,
    /// Ranked repositories.
    pub repositories: Vec<RepositoryScore>,
    /// Ranked teams. Teams without selected signals retain a zero-point row.
    pub teams: Vec<TeamScore>,
}

/// Cache counters for one organization. Misses count successful computations only.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
pub struct CacheStats {
    /// Successful queries served from the cache.
    pub hits: u64,
    /// Successful queries computed from the current signal snapshot.
    pub misses: u64,
    /// Stored results.
    pub entries: usize,
}

struct Entry {
    query: ScoreQuery,
    result: Scoreboards,
}

#[derive(Default)]
struct Snapshot {
    signals: Vec<Signal>,
    memberships: Vec<TeamMembership>,
}

#[derive(Default)]
struct Cache {
    entries: VecDeque<Entry>,
    hits: u64,
    misses: u64,
}

#[derive(Default)]
struct Organization {
    snapshot: Snapshot,
    presets: HashMap<String, ScoreQuery>,
    cache: Cache,
}

fn score<'a>(
    snapshot: &Snapshot,
    cache: &'a mut Cache,
    query: &ScoreQuery,
    capacity: usize,
) -> Result<&'a Scoreboards, ScoringError> {
    if let Some(index) = cache.entries.iter().position(|entry| entry.query == *query) {
        cache.hits = cache.hits.saturating_add(1);
        return Ok(&cache.entries[index].result);
    }
    let bounds = query.period.bounds()?;
    let mut selected = Vec::new();
    for signal in &snapshot.signals {
        if !query.filter.includes(signal) {
            continue;
        }
        if let Some((start, end)) = bounds {
            let timestamp = DateTime::parse_from_rfc3339(&signal.event_timestamp)
                .ok()
                .filter(|time| time.nanosecond() < 1_000_000_000)
                .ok_or_else(|| ScoringError::InvalidTimestamp(signal.event_timestamp.clone()))?
                .to_utc();
            if timestamp < start || timestamp >= end {
                continue;
            }
        }
        selected.push(signal);
    }
    let result = scoreboards(&selected, &query.rules, &snapshot.memberships)?;
    if cache.entries.len() == capacity {
        cache.entries.pop_front();
    }
    cache.entries.push_back(Entry {
        query: query.clone(),
        result,
    });
    cache.misses = cache.misses.saturating_add(1);
    let Some(entry) = cache.entries.back() else {
        unreachable!("the newly inserted cache entry must exist");
    };
    Ok(&entry.result)
}

/// Holds independent signal snapshots, named presets, and bounded caches per organization.
/// Callers must supply only signals and memberships from the named organization.
pub struct ScoringCatalog {
    organizations: HashMap<String, Organization>,
    capacity: NonZeroUsize,
}

impl Default for ScoringCatalog {
    fn default() -> Self {
        Self::new(NonZeroUsize::new(32).unwrap_or(NonZeroUsize::MIN))
    }
}

impl ScoringCatalog {
    /// Create a catalog with a cache capacity per organization.
    #[must_use]
    pub fn new(capacity: NonZeroUsize) -> Self {
        Self {
            organizations: HashMap::new(),
            capacity,
        }
    }

    /// Replace an organization's input snapshot and invalidate its cached results.
    /// Presets and other organizations remain unchanged.
    ///
    /// # Errors
    /// Returns an error when the organization name is empty.
    pub fn replace_organization(
        &mut self,
        organization: &str,
        signals: Vec<Signal>,
        memberships: Vec<TeamMembership>,
    ) -> Result<(), ScoringError> {
        if organization.trim().is_empty() {
            return Err(ScoringError::InvalidOrganization);
        }
        let org = self.organizations.entry(organization.to_owned()).or_default();
        org.snapshot.signals = signals;
        org.snapshot.memberships = memberships;
        org.cache.entries.clear();
        Ok(())
    }

    /// Save or replace a named preset within one organization.
    ///
    /// # Errors
    /// Returns an error when the organization is unknown, name empty, or query invalid.
    pub fn save_preset(&mut self, organization: &str, name: &str, query: ScoreQuery) -> Result<(), ScoringError> {
        if name.trim().is_empty() {
            return Err(ScoringError::InvalidPreset);
        }
        query.validate()?;
        self.organization_mut(organization)?
            .presets
            .insert(name.to_owned(), query);
        Ok(())
    }

    /// Return cached rankings or score the current snapshot using these options.
    ///
    /// # Errors
    /// Returns an error for unknown organizations, invalid periods, timestamps, or rules.
    pub fn query(&mut self, organization: &str, options: &ScoreQuery) -> Result<&Scoreboards, ScoringError> {
        let capacity = self.capacity.get();
        let org = self.organization_mut(organization)?;
        score(&org.snapshot, &mut org.cache, options, capacity)
    }

    /// Score an organization with one of its named presets.
    ///
    /// # Errors
    /// Returns an error for unknown organizations or presets, or invalid scoring input.
    pub fn query_preset(&mut self, organization: &str, name: &str) -> Result<&Scoreboards, ScoringError> {
        let capacity = self.capacity.get();
        let org = self.organization_mut(organization)?;
        let query = org
            .presets
            .get(name)
            .ok_or_else(|| ScoringError::UnknownPreset(name.to_owned()))?;
        score(&org.snapshot, &mut org.cache, query, capacity)
    }

    /// Read cache counters for an organization.
    ///
    /// # Errors
    /// Returns an error when the organization is unknown.
    pub fn cache_stats(&self, organization: &str) -> Result<CacheStats, ScoringError> {
        let org = self
            .organizations
            .get(organization)
            .ok_or_else(|| ScoringError::UnknownOrganization(organization.to_owned()))?;
        Ok(CacheStats {
            hits: org.cache.hits,
            misses: org.cache.misses,
            entries: org.cache.entries.len(),
        })
    }

    /// Invalidate cached results for an organization without changing its data or presets.
    ///
    /// # Errors
    /// Returns an error when the organization is unknown.
    pub fn clear_cache(&mut self, organization: &str) -> Result<(), ScoringError> {
        self.organization_mut(organization)?.cache.entries.clear();
        Ok(())
    }

    fn organization_mut(&mut self, name: &str) -> Result<&mut Organization, ScoringError> {
        self.organizations
            .get_mut(name)
            .ok_or_else(|| ScoringError::UnknownOrganization(name.to_owned()))
    }
}
