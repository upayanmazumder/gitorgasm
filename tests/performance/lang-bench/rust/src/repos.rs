use serde::Deserialize;

use crate::timestamp::parse_timestamp;
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
