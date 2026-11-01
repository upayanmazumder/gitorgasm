use std::collections::VecDeque;

use super::CacheStats;
use crate::ScoringError;
use crate::engine::scoreboards;
use crate::model::{Signal, TeamMembership};
use crate::query::{ScoreQuery, Scoreboards};

struct Entry {
    query: ScoreQuery,
    result: Scoreboards,
}

#[derive(Default)]
pub(super) struct Snapshot {
    pub(super) signals: Vec<Signal>,
    pub(super) memberships: Vec<TeamMembership>,
}

#[derive(Default)]
pub(super) struct Cache {
    entries: VecDeque<Entry>,
    hits: u64,
    misses: u64,
}

impl Cache {
    pub(super) fn clear(&mut self) {
        self.entries.clear();
    }

    pub(super) fn stats(&self) -> CacheStats {
        CacheStats {
            hits: self.hits,
            misses: self.misses,
            entries: self.entries.len(),
        }
    }
}

pub(super) fn score<'a>(
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
            let timestamp = signal.timestamp()?;
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
