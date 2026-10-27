//! Scoring errors.

/// Scoring failure. The crate can add variants, so match with a wildcard arm.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
#[non_exhaustive]
pub enum ScoringError {
    /// Invalid RFC 3339 timestamp or leap second.
    #[error("invalid RFC 3339 timestamp {0:?}")]
    InvalidTimestamp(String),
    /// NaN or infinite ruleset field, named in camelCase.
    #[error("ruleset field {0} is not a finite number")]
    NonFiniteRule(&'static str),
    /// A score total overflowed to infinity or NaN.
    #[error("a score total is not a finite number")]
    NonFiniteTotal,
    /// A ruleset value is outside its supported nonnegative range.
    #[error("invalid ruleset field {0}")]
    InvalidRule(&'static str),
    /// Requested organization was not loaded.
    #[error("unknown organization {0:?}")]
    UnknownOrganization(String),
    /// Empty organization name.
    #[error("organization name must not be empty")]
    InvalidOrganization,
    /// Requested named preset was not saved for this organization.
    #[error("unknown scoring preset {0:?}")]
    UnknownPreset(String),
    /// Empty preset name.
    #[error("preset name must not be empty")]
    InvalidPreset,
    /// The requested period has no positive duration or lies outside supported dates.
    #[error("invalid scoring period")]
    InvalidPeriod,
}
