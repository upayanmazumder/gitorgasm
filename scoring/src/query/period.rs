use chrono::{DateTime, Datelike, Days, TimeDelta, Utc};
use serde::{Deserialize, Serialize};

use crate::ScoringError;

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
    pub(super) fn bounds(&self) -> Result<Option<TimeBounds>, ScoringError> {
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
