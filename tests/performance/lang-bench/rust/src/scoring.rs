use std::collections::HashMap;

use crate::rng::SplitMix64;
pub const NUM_KINDS: usize = 10;
pub const NUM_USERS: u64 = 50_000;
pub const NUM_REPOS: u64 = 10_000;
pub const SPAN_SEC: i64 = 180 * 86400;
pub const WINDOW_SEC: i64 = 90 * 86400;
pub const BASE_TS: i64 = 1_767_225_600; // 2026-01-01T00:00:00Z
pub const TOP_N: usize = 100;

/// Weights for commit, PR open/merge, review, issue open/close, comment, release, star, discussion.
pub const WEIGHTS: [f64; NUM_KINDS] = [1.0, 3.0, 5.0, 4.0, 2.0, 2.0, 0.5, 6.0, 0.2, 1.0];

pub struct Event {
    pub ts: i64,
    pub user_id: u32,
    pub repo_id: u32,
    pub kind: u8,
}

/// Same draw order as Go `GenerateEvents`.
pub fn generate_events(n: usize, seed: u64) -> Vec<Event> {
    let mut r = SplitMix64::new(seed);
    (0..n)
        .map(|_| {
            let (a, b) = (r.next_u64() % NUM_USERS, r.next_u64() % NUM_USERS);
            let user_id = a.min(b) as u32;
            let repo_id = (r.next_u64() % NUM_REPOS) as u32;
            let kind = (r.next_u64() % NUM_KINDS as u64) as u8;
            let ts = BASE_TS + (r.next_u64() % SPAN_SEC as u64) as i64;
            Event {
                ts,
                user_id,
                repo_id,
                kind,
            }
        })
        .collect()
}

#[derive(Default)]
struct UserState {
    counts: [u32; NUM_KINDS],
    score: f64,
}

#[derive(Clone, Copy)]
pub struct UserScore {
    pub user_id: u32,
    pub score: f64,
}

/// Uses the default `HashMap` hasher to benchmark ordinary Rust code.
pub fn score(events: &[Event], now: i64) -> Vec<UserScore> {
    let since = now - WINDOW_SEC;
    let mut users: HashMap<u32, UserState> = HashMap::new();
    for e in events {
        if e.ts < since || e.ts > now {
            continue;
        }
        let s = users.entry(e.user_id).or_default();
        let k = e.kind as usize;
        let n = s.counts[k];
        s.score += WEIGHTS[k] / (1.0 + 0.1 * n as f64);
        s.counts[k] = n + 1;
    }
    let mut all: Vec<UserScore> = users
        .into_iter()
        .map(|(user_id, s)| UserScore {
            user_id,
            score: s.score,
        })
        .collect();
    all.sort_unstable_by(|a, b| b.score.total_cmp(&a.score).then(a.user_id.cmp(&b.user_id)));
    all.truncate(TOP_N);
    all
}

/// Same formula as Go `ScoreChecksum`.
pub fn score_checksum(top: &[UserScore]) -> (u64, f64) {
    let mut ids = 0u64;
    let mut sum = 0f64;
    for (i, u) in top.iter().enumerate() {
        ids += u.user_id as u64 * (i as u64 + 1);
        sum += u.score;
    }
    (ids, sum)
}
