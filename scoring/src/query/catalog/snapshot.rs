use crate::ScoringError;
use crate::engine::scoreboards;
use crate::model::{Signal, TeamMembership};
use crate::query::{ScoreQuery, Scoreboards};

#[derive(Default)]
pub(super) struct Snapshot {
    pub(super) signals: Vec<Signal>,
    pub(super) memberships: Vec<TeamMembership>,
}

impl Snapshot {
    pub(super) fn score(&self, query: &ScoreQuery) -> Result<Scoreboards, ScoringError> {
        let bounds = query.period.bounds()?;
        let mut selected = Vec::new();
        for signal in &self.signals {
            if !query.filter.includes(signal) {
                continue;
            }
            if let Some((start, end)) = bounds {
                let timestamp = signal.timestamp()?;
                if timestamp < start || timestamp >= end {
                    continue;
                }
            }
            selected.push(signal);
        }
        scoreboards(selected.into_iter(), &query.rules, &self.memberships)
    }
}
