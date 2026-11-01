//! Rust language benchmark; keep behavior aligned with `../go/lb`.

mod repos;
mod rng;
mod scoring;
mod timestamp;

pub use repos::{
    FLAG_ARCHIVED, FLAG_FORK, FLAG_PRIVATE, OwnerJson, PERM_ADMIN, PERM_PULL, PERM_PUSH, PermJson,
    RepoJson, RepoRecord, parse_repos, repo_checksum,
};
pub use rng::SplitMix64;
pub use scoring::{
    BASE_TS, Event, NUM_KINDS, NUM_REPOS, NUM_USERS, SPAN_SEC, TOP_N, UserScore, WEIGHTS,
    WINDOW_SEC, generate_events, score, score_checksum,
};
pub use timestamp::parse_timestamp;

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
