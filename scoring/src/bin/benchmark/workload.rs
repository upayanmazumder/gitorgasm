use gitorgasm_scoring::{Metadata, Signal, SignalType};

pub(super) const EVENTS: usize = 100_000;

pub(super) fn signals() -> Vec<Signal> {
    let kinds = [
        SignalType::Commit,
        SignalType::PrOpen,
        SignalType::PrMerge,
        SignalType::Review,
        SignalType::IssueOpen,
        SignalType::IssueClose,
    ];
    let mut signals = Vec::with_capacity(EVENTS);
    for i in 0..EVENTS {
        // Twenty-eight days and stable hour/second distribution; no clock or random input.
        let day = i % 28 + 1;
        let hour = i / 28 % 24;
        let second = i / (28 * 24) % 60;
        signals.push(Signal {
            user_id: i64::try_from(i % 200).expect("the remainder is below 200") + 1,
            user_login: format!("user-{}", i % 200 + 1),
            kind: kinds[(i / 200) % kinds.len()],
            repo: format!("repo-{}", (i / 200) % 40 + 1),
            event_timestamp: format!("2026-01-{day:02}T{hour:02}:00:{second:02}Z"),
            metadata: Metadata::default(),
        });
    }
    signals
}
