//! Scoring errors.

/// Scoring failure. The crate can add variants, so match with a wildcard arm.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
#[non_exhaustive]
pub enum ScoringError {
    /// Invalid RFC 3339 timestamp or leap second (legacy `RangeError`).
    #[error("invalid RFC 3339 timestamp {0:?}")]
    InvalidTimestamp(String),
    /// NaN or infinite ruleset field, named in camelCase.
    #[error("ruleset field {0} is not a finite number")]
    NonFiniteRule(&'static str),
    /// A score total overflowed to infinity or NaN. Legacy returned the non-finite value.
    #[error("a score total is not a finite number")]
    NonFiniteTotal,
}
