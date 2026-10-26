//! Runs one workload once, for peak-RSS measurement.
//!
//!     standalone repos <in.json>   parse + normalize the fixture
//!     standalone score             generate 1M events, score, top-100

use langbench::{
    BASE_TS, SPAN_SEC, generate_events, parse_repos, repo_checksum, score, score_checksum,
};
use std::time::Instant;

fn main() {
    let args: Vec<String> = std::env::args().collect();
    let start = Instant::now();
    match args.get(1).map(String::as_str) {
        Some("repos") => {
            let path = args.get(2).expect("missing path argument");
            let data = std::fs::read(path).expect("read fixture");
            let recs = parse_repos(&data).expect("parse");
            println!("repos={} checksum={}", recs.len(), repo_checksum(&recs));
        }
        Some("score") => {
            let events = generate_events(1_000_000, 7);
            let top = score(&events, BASE_TS + SPAN_SEC);
            let (ids, sum) = score_checksum(&top);
            println!(
                "top={} first={}:{:.6} id_checksum={ids} score_sum={sum:.6}",
                top.len(),
                top[0].user_id,
                top[0].score
            );
        }
        _ => {
            eprintln!("usage: standalone repos <path> | score");
            std::process::exit(1);
        }
    }
    println!("elapsed={:?} {}", start.elapsed(), vm_hwm());
}

fn vm_hwm() -> String {
    std::fs::read_to_string("/proc/self/status")
        .ok()
        .and_then(|s| {
            s.lines()
                .find(|l| l.starts_with("VmHWM:"))
                .map(|l| l.split_whitespace().skip(1).collect::<String>())
        })
        .map_or("VmHWM=?".into(), |v| format!("VmHWM={v}"))
}
