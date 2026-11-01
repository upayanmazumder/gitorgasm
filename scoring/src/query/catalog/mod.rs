use std::collections::HashMap;
use std::num::NonZeroUsize;

use super::{ScoreQuery, Scoreboards};
use crate::ScoringError;
use crate::model::{Signal, TeamMembership};

mod cache;

use cache::{Cache, Snapshot, score};

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

#[derive(Default)]
struct Organization {
    snapshot: Snapshot,
    presets: HashMap<String, ScoreQuery>,
    cache: Cache,
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
        org.cache.clear();
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
        Ok(org.cache.stats())
    }

    /// Invalidate cached results for an organization without changing its data or presets.
    ///
    /// # Errors
    /// Returns an error when the organization is unknown.
    pub fn clear_cache(&mut self, organization: &str) -> Result<(), ScoringError> {
        self.organization_mut(organization)?.cache.clear();
        Ok(())
    }

    fn organization_mut(&mut self, name: &str) -> Result<&mut Organization, ScoringError> {
        self.organizations
            .get_mut(name)
            .ok_or_else(|| ScoringError::UnknownOrganization(name.to_owned()))
    }
}
