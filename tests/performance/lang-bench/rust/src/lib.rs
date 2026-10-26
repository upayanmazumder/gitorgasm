//! Rust language benchmark; keep behavior aligned with `../go/lb`.

use serde::Deserialize;
use std::collections::HashMap;

/// SplitMix64, identical to the Go implementation.
pub struct SplitMix64(u64);

impl SplitMix64 {
    pub fn new(seed: u64) -> Self {
        Self(seed)
    }

    pub fn next_u64(&mut self) -> u64 {
        self.0 = self.0.wrapping_add(0x9E37_79B9_7F4A_7C15);
        let mut z = self.0;
        z = (z ^ (z >> 30)).wrapping_mul(0xBF58_476D_1CE4_E5B9);
        z = (z ^ (z >> 27)).wrapping_mul(0x94D0_49BB_1331_11EB);
        z ^ (z >> 31)
    }
}

// Workload 1: parse and normalize GitHub organization repositories.

#[derive(Deserialize)]
pub struct OwnerJson {
    pub login: String,
    pub id: i64,
}

#[derive(Deserialize)]
pub struct PermJson {
    pub admin: bool,
    pub push: bool,
    pub pull: bool,
}

/// Only the fields Gitorgasm reads. serde skips unknown keys.
#[derive(Deserialize)]
pub struct RepoJson {
    pub id: i64,
    pub node_id: String,
    pub name: String,
    pub full_name: String,
    pub private: bool,
    pub owner: OwnerJson,
    pub description: Option<String>,
    pub fork: bool,
    pub archived: bool,
    pub visibility: String,
    pub default_branch: String,
    pub pushed_at: String,
    pub updated_at: String,
    pub stargazers_count: i64,
    pub language: Option<String>,
    #[serde(default)]
    pub topics: Vec<String>,
    pub permissions: Option<PermJson>,
}

pub const FLAG_PRIVATE: u8 = 1 << 0;
pub const FLAG_FORK: u8 = 1 << 1;
pub const FLAG_ARCHIVED: u8 = 1 << 2;
pub const PERM_ADMIN: u8 = 1 << 0;
pub const PERM_PUSH: u8 = 1 << 1;
pub const PERM_PULL: u8 = 1 << 2;

/// Compact internal form.
pub struct RepoRecord {
    pub id: i64,
    pub pushed_at: i64,
    pub updated_at: i64,
    pub name: String,
    pub owner: String,
    pub default_branch: String,
    pub language: String,
    pub topics: Vec<String>,
    pub stars: i32,
    pub flags: u8,
    pub visibility: u8, // 0 public, 1 private, 2 internal
    pub perm: u8,
}

pub fn parse_repos(data: &[u8]) -> Result<Vec<RepoRecord>, String> {
    let raw: Vec<RepoJson> = serde_json::from_slice(data).map_err(|e| e.to_string())?;
    raw.into_iter().map(normalize).collect()
}

fn normalize(r: RepoJson) -> Result<RepoRecord, String> {
    let mut flags = 0;
    if r.private {
        flags |= FLAG_PRIVATE;
    }
    if r.fork {
        flags |= FLAG_FORK;
    }
    if r.archived {
        flags |= FLAG_ARCHIVED;
    }
    let visibility = match r.visibility.as_str() {
        "public" => 0,
        "private" => 1,
        "internal" => 2,
        v => return Err(format!("repo {}: unknown visibility {v:?}", r.id)),
    };
    let mut perm = 0;
    if let Some(p) = &r.permissions {
        if p.admin {
            perm |= PERM_ADMIN;
        }
        if p.push {
            perm |= PERM_PUSH;
        }
        if p.pull {
            perm |= PERM_PULL;
        }
    }
    Ok(RepoRecord {
        id: r.id,
        pushed_at: parse_timestamp(&r.pushed_at)?,
        updated_at: parse_timestamp(&r.updated_at)?,
        name: r.name,
        owner: r.owner.login,
        default_branch: r.default_branch,
        language: r.language.unwrap_or_default(),
        topics: r.topics,
        stars: r.stargazers_count as i32,
        flags,
        visibility,
        perm,
    })
}

/// Same formula as Go `RepoChecksum`.
pub fn repo_checksum(recs: &[RepoRecord]) -> u64 {
    let mut s: u64 = 0;
    for r in recs {
        s = s
            .wrapping_add(r.id as u64)
            .wrapping_add(r.pushed_at as u64)
            .wrapping_add(r.updated_at as u64)
            .wrapping_add(r.stars as u64);
        s = s.wrapping_add((r.flags as u64) << 8 | (r.visibility as u64) << 4 | r.perm as u64);
        s = s
            .wrapping_add(r.topics.len() as u64)
            .wrapping_add(r.name.len() as u64)
            .wrapping_add(r.language.len() as u64);
    }
    s
}

/// Parses GitHub's fixed "YYYY-MM-DDTHH:MM:SSZ" layout like the Go benchmark.
pub fn parse_timestamp(s: &str) -> Result<i64, String> {
    let b = s.as_bytes();
    if b.len() != 20
        || b[4] != b'-'
        || b[7] != b'-'
        || b[10] != b'T'
        || b[13] != b':'
        || b[16] != b':'
        || b[19] != b'Z'
    {
        return Err(format!("bad timestamp {s:?}"));
    }
    let num = |a: usize, e: usize| -> Result<i64, String> {
        let mut v = 0i64;
        for &c in &b[a..e] {
            if !c.is_ascii_digit() {
                return Err(format!("bad timestamp {s:?}"));
            }
            v = v * 10 + (c - b'0') as i64;
        }
        Ok(v)
    };
    let (y, mo, d) = (num(0, 4)?, num(5, 7)?, num(8, 10)?);
    let (h, mi, se) = (num(11, 13)?, num(14, 16)?, num(17, 19)?);
    Ok(days_from_civil(y, mo, d) * 86400 + h * 3600 + mi * 60 + se)
}

/// Howard Hinnant's algorithm.
fn days_from_civil(mut y: i64, m: i64, d: i64) -> i64 {
    if m <= 2 {
        y -= 1;
    }
    let era = y.div_euclid(400);
    let yoe = y - era * 400;
    let mp = (m + 9) % 12;
    let doy = (153 * mp + 2) / 5 + d - 1;
    let doe = yoe * 365 + yoe / 4 - yoe / 100 + doy;
    era * 146097 + doe - 719468
}

// Workload 2: contributor scoring with diminishing returns and top 100.

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

#[cfg(test)]
mod tests {
    #[test]
    fn timestamp() {
        assert_eq!(
            super::parse_timestamp("2026-01-01T00:00:00Z"),
            Ok(super::BASE_TS)
        );
    }
}
