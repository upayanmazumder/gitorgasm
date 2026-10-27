//! Checks the score process protocol and query validation.

use std::io::Write;
use std::process::{Command, Stdio};

use serde_json::{Value, json};

fn invoke(input: &Value) -> std::process::Output {
    invoke_mode(input, None)
}

fn invoke_mode(input: &Value, mode: Option<&str>) -> std::process::Output {
    let mut command = Command::new(env!("CARGO_BIN_EXE_score"));
    if let Some(mode) = mode {
        command.arg(mode);
    }
    let mut child = command
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .unwrap();
    serde_json::to_writer(child.stdin.as_mut().unwrap(), input).unwrap();
    child.stdin.take().unwrap().flush().unwrap();
    child.wait_with_output().unwrap()
}

fn event(kind: &str, at: &str) -> Value {
    json!({"userId": 42, "userLogin": "alice", "type": kind,
        "repo": "api", "eventTimestamp": at, "metadata": {"isBot": false}})
}

#[test]
fn accepts_full_envelope_and_emits_complete_json_scoreboards() {
    let input = json!({"organization":"1234",
        "signals":[event("commit", "2026-01-05T10:00:00Z"),event("review", "2026-01-06T10:00:00Z")],
        "memberships":[{"userId":42,"teamSlug":"dev","teamName":"Development","repositoryNames":["api"]}],
        "query":{"period":{"kind":"custom","start":"2026-01-05T00:00:00Z","end":"2026-01-06T00:00:00Z"},
            "filter":{"preset":"code","excludeBots":true,"users":[42],"repositories":["api"],"kinds":["commit"],"search":"ALICE"}}
    });
    let output = invoke(&input);
    assert!(output.status.success(), "{}", String::from_utf8_lossy(&output.stderr));
    let data: Value = serde_json::from_slice(&output.stdout).unwrap();
    assert_eq!(data["contributors"][0]["userId"], 42);
    assert_eq!(data["contributors"][0]["score"], 1.0);
    assert_eq!(data["contributors"][0]["counts"]["commits"], 1);
    assert_eq!(data["repositories"][0]["repoName"], "api");
    assert_eq!(data["teams"][0]["teamSlug"], "dev");
    assert_eq!(data["teams"][0]["score"], 1.0);
}

#[test]
fn defaults_query_and_accepts_empty_memberships() {
    let output = invoke(
        &json!({"organization":"account", "signals":[event("pr_open", "2026-01-05T10:00:00Z")], "memberships":[], "query":{}}),
    );
    assert!(output.status.success(), "{}", String::from_utf8_lossy(&output.stderr));
    let data: Value = serde_json::from_slice(&output.stdout).unwrap();
    assert_eq!(data["contributors"][0]["score"], 4.0);
    assert_eq!(data["teams"], json!([]));
}

#[test]
fn invalid_period_event_and_schema_fail_without_success_json() {
    let base = json!({"organization":"org", "signals":[], "memberships":[],
        "query":{"period":{"kind":"preset","preset":"today"}}});
    let output = invoke(&base);
    assert_eq!(output.status.code(), Some(2));
    assert_eq!(String::from_utf8_lossy(&output.stdout), "");
    assert_ne!(String::from_utf8_lossy(&output.stderr), "");

    let output =
        invoke(&json!({"organization":"org", "signals":[event("commit", "invalid")], "memberships":[], "query":{}}));
    assert_eq!(output.status.code(), Some(2));
    assert_eq!(String::from_utf8_lossy(&output.stdout), "");

    let output = invoke(&json!({"organization":"", "signals":[], "memberships":[], "query":{}}));
    assert_eq!(output.status.code(), Some(2));
    assert_eq!(String::from_utf8_lossy(&output.stdout), "");
}

#[test]
fn validates_bare_query_without_scoring() {
    let output = invoke_mode(&json!({}), Some("--validate-query"));
    assert!(output.status.success(), "{}", String::from_utf8_lossy(&output.stderr));
    assert_eq!(String::from_utf8_lossy(&output.stdout), "");

    let output = invoke_mode(
        &json!({"period":{"kind":"custom",
        "start":"2026-01-05T00:00:00Z","end":"2026-01-05T00:00:00Z"}}),
        Some("--validate-query"),
    );
    assert_eq!(output.status.code(), Some(2));
    assert_eq!(String::from_utf8_lossy(&output.stdout), "");

    let output = invoke_mode(&json!({"filter":{"preset":"not_a_filter"}}), Some("--validate-query"));
    assert_eq!(output.status.code(), Some(2));
    assert_eq!(String::from_utf8_lossy(&output.stdout), "");
}
