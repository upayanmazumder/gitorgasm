use std::collections::VecDeque;

use super::CacheStats;
use super::snapshot::Snapshot;
use crate::ScoringError;
use crate::query::{ScoreQuery, Scoreboards};

struct Entry {
    query: ScoreQuery,
    result: Scoreboards,
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
    let result = snapshot.score(query)?;
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
