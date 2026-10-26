//! Release-mode timing harness with warmups, percentiles, and untimed allocation counts.

use langbench::{
    BASE_TS, SPAN_SEC, TOP_N, generate_events, parse_repos, repo_checksum, score, score_checksum,
};
use std::alloc::{GlobalAlloc, Layout, System};
use std::hint::black_box;
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering::Relaxed};
use std::time::{Duration, Instant};

/// Counts allocations only in an untimed run, leaving one relaxed load per timed allocation.
struct Counting;
static COUNTING: AtomicBool = AtomicBool::new(false);
static ALLOC_BYTES: AtomicU64 = AtomicU64::new(0);
static ALLOC_COUNT: AtomicU64 = AtomicU64::new(0);

fn record(size: usize) {
    if COUNTING.load(Relaxed) {
        ALLOC_BYTES.fetch_add(size as u64, Relaxed);
        ALLOC_COUNT.fetch_add(1, Relaxed);
    }
}

unsafe impl GlobalAlloc for Counting {
    unsafe fn alloc(&self, l: Layout) -> *mut u8 {
        record(l.size());
        unsafe { System.alloc(l) }
    }
    unsafe fn dealloc(&self, p: *mut u8, l: Layout) {
        unsafe { System.dealloc(p, l) }
    }
    unsafe fn realloc(&self, p: *mut u8, l: Layout, new: usize) -> *mut u8 {
        record(new);
        unsafe { System.realloc(p, l, new) }
    }
}

#[global_allocator]
static GLOBAL: Counting = Counting;

const WARMUPS: usize = 5;

fn run<T>(name: &str, iters: usize, mut f: impl FnMut() -> T) {
    for _ in 0..WARMUPS {
        black_box(f());
    }
    let mut times: Vec<Duration> = Vec::with_capacity(iters);
    for _ in 0..iters {
        let t = Instant::now();
        black_box(f());
        times.push(t.elapsed());
    }
    let (b0, c0) = (ALLOC_BYTES.load(Relaxed), ALLOC_COUNT.load(Relaxed));
    COUNTING.store(true, Relaxed);
    black_box(f());
    COUNTING.store(false, Relaxed);
    let bytes = ALLOC_BYTES.load(Relaxed) - b0;
    let count = ALLOC_COUNT.load(Relaxed) - c0;
    times.sort();
    let pct = |p: f64| times[((iters - 1) as f64 * p).round() as usize];
    let mean = times.iter().sum::<Duration>() / iters as u32;
    println!(
        "{name:<12} iters={iters} median={:?} p95={:?} mean={:?} min={:?} B/op={bytes} allocs/op={count}",
        pct(0.5),
        pct(0.95),
        mean,
        times[0]
    );
}

fn main() {
    let args: Vec<String> = std::env::args().collect();
    let path = args
        .get(1)
        .map(String::as_str)
        .unwrap_or("../fixtures/repos.json");
    let iters: usize = args.get(2).and_then(|s| s.parse().ok()).unwrap_or(50);

    let data = std::fs::read(path).expect("read fixture");
    let recs = parse_repos(&data).expect("parse");
    println!(
        "fixture bytes={} repos={} checksum={}",
        data.len(),
        recs.len(),
        repo_checksum(&recs)
    );
    drop(recs);
    run("ParseRepos", iters, || parse_repos(&data).expect("parse"));

    let events = generate_events(1_000_000, 7);
    let top = score(&events, BASE_TS + SPAN_SEC);
    assert_eq!(top.len(), TOP_N);
    let (ids, sum) = score_checksum(&top);
    println!(
        "score first={}:{:.6} id_checksum={ids} score_sum={sum:.6}",
        top[0].user_id, top[0].score
    );
    run("Score", iters, || score(&events, BASE_TS + SPAN_SEC));
}
