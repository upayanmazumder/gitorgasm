use std::io;

use gitorgasm_scoring::{ScoreQuery, Signal, TeamMembership};
use serde::Deserialize;

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
pub(super) struct Request {
    pub(super) organization: String,
    pub(super) signals: Vec<Signal>,
    pub(super) memberships: Vec<TeamMembership>,
    pub(super) query: ScoreQuery,
}

pub(super) enum Failure {
    Input(String),
    Internal(String),
}

impl From<serde_json::Error> for Failure {
    fn from(error: serde_json::Error) -> Self {
        if error.is_io() {
            Self::Internal(error.to_string())
        } else {
            Self::Input(error.to_string())
        }
    }
}

impl From<gitorgasm_scoring::ScoringError> for Failure {
    fn from(error: gitorgasm_scoring::ScoringError) -> Self {
        Self::Input(error.to_string())
    }
}

impl From<io::Error> for Failure {
    fn from(error: io::Error) -> Self {
        Self::Internal(error.to_string())
    }
}
