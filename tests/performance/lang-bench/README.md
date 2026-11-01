# Go vs Rust language benchmark

This benchmark compares Go and Rust on two workloads that Gitorgasm runs. It also compares the memory and size cost of a minimal HTTP service. Results are in [RESULTS.md](RESULTS.md). Raw logs are in [results/](results/).

## Purpose

**Decision** input: choose the backend language for Gitorgasm services. The target cluster is a single k3s node with 4 vCPU and about 7.6 GiB RAM. Memory requests are already about 79% allocated. Memory per pod is therefore the main constraint, then CPU time.

## Layout

| Path                         | Content                                                                   |
| ---------------------------- | ------------------------------------------------------------------------- |
| `go/lb/`                     | Go workloads, fixture generator, PRNG, `testing.B` benchmarks             |
| `go/cmd/standalone/`         | Go one-shot runner for peak RSS. Also writes the fixture (`gen` mode).    |
| `go/cmd/hello/`              | Go `net/http` hello service                                               |
| `go/cmd/loadclient/`         | Go HTTP client. Sends N sequential requests on one keep-alive connection. |
| `rust/src/lib.rs`            | Public exports for repository, timestamp, scoring, and PRNG modules      |
| `rust/src/bin/bench.rs`      | Rust timing harness                                                       |
| `rust/src/bin/standalone.rs` | Rust one-shot runner for peak RSS                                         |
| `rust/src/bin/hello.rs`      | Rust `axum` + `tokio` hello service                                       |
| `run.sh`                     | Builds everything and runs all measurements. Prints a raw log.            |
| `fixtures/repos.json`        | Generated fixture. Not committed.                                         |

## Workloads

### 1. Repository list parse (`ParseRepos`)

- Input: one JSON array of 10,000 repository objects in the shape of GitHub `GET /orgs/{org}/repos`. Size: 9,784,235 bytes.
- Each object has the fields that Gitorgasm reads: `id`, `node_id`, `name`, `full_name`, `private`, `owner{login,id}`, `description`, `fork`, `archived`, `visibility`, `default_branch`, `pushed_at`, `updated_at`, `stargazers_count`, `language`, `topics[]`, `permissions{admin,push,pull}`.
- Each object also has fields that Gitorgasm ignores (`html_url`, `url`, `clone_url`, `created_at`, `size`, counts, `license{}`, extra `owner` fields). The decoder must skip them, as it must for the real API.
- The Go generator (`go/lb/fixture.go`) writes the fixture with a seeded PRNG (seed 42). Both languages read the same file. `run.sh` prints its SHA-256.
- Step 1: decode into typed structs. Go uses `encoding/json` (v1) and `encoding/json/v2`. Both are in the Go 1.27 standard library. Rust uses `serde_json::from_slice` with `#[derive(Deserialize)]` and owned `String` fields.
- Step 2: normalize each object into a compact record. Booleans become bit flags. `visibility` becomes an enum byte. Timestamps become Unix seconds. Both languages use the same hand-written timestamp parser, so the comparison measures JSON decoding and not date libraries.
- The real API returns at most 100 objects per page. 10,000 objects in one array is a stress size. Per-page cost is about 1/100 of the measured cost.

### 2. Contributor scoring (`Score`)

- Input: 1,000,000 synthetic signal events `(user_id, repo_id, kind, timestamp)`. 50,000 users, 10,000 repos, 10 kinds, 180-day span. User ids are skewed toward low ids (minimum of two uniform draws). Both languages generate identical events with SplitMix64 (seed 7). Generation is not timed.
- Only events in the last 90 days count (about half).
- Each counted event adds `weight[kind] / (1 + 0.1 * n)`. `n` is the number of earlier counted events of that kind for that user (diminishing returns).
- Per-user state lives in a hash map. Go uses `map[uint32]*userState`. Rust uses `std::collections::HashMap` with the default SipHash hasher. These are the defaults that plain code in each language uses.
- Output: all users sorted by score (descending), then user id (ascending). The top 100 are kept.

### Equivalence check

Both languages print checksums of their output (`RepoChecksum`, `ScoreChecksum`). `run.sh` prints them in the `equivalence` section. They must match exactly, including the floating-point score sum.

## Measurements

| Metric             | Go method                                                                                                                        | Rust method                                                                          |
| ------------------ | -------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------ |
| Time per op        | `go test -bench . -benchmem -cpu 1,16 -count 8`. Median of the 8 `ns/op` values.                                                 | `bench` binary: 5 warmup runs, then 100 timed runs. Median and p95 of per-run times. |
| Allocations per op | `-benchmem` (`B/op`, `allocs/op`)                                                                                                | Counting global allocator, active only for one extra untimed run                     |
| Peak RSS           | `/usr/bin/time -v` "Maximum resident set size" of `standalone`, 5 runs, median                                                   | Same                                                                                 |
| Binary size        | `go build` default and `-ldflags='-s -w'`                                                                                        | `cargo build --release` default profile, and `strip` of the same binary              |
| Idle service RSS   | `VmRSS`/`VmHWM` from `/proc/<pid>/status` 1 s after start, and 1 s after 10,000 sequential keep-alive requests from `loadclient` | Same                                                                                 |

Notes on method:

- `-cpu 1` runs Go with `GOMAXPROCS=1`. This is closest to a single-threaded Rust run. `-cpu 16` lets the Go garbage collector use other cores in parallel. The Rust code is single-threaded in both workloads.
- Peak RSS of `standalone` includes the input. For `repos`, that is the 9.8 MB file in memory. For `score`, that is the 1M-event array (24 MB in both languages).
- `oha`, `hey`, `wrk`, and `curl`-based loops are not used. `loadclient` is a small Go program, so the client is the same for both servers.
- The Rust harness does not use criterion. The harness has no dependencies beyond the crate itself, so it builds offline.

## Run

Requirements: Go 1.27 or later, Rust 1.85 or later (edition 2024), `/usr/bin/time`, `strip`. The Rust build uses `--offline`. Remove that flag if the crates are not in the local Cargo cache.

```sh
cd tests/performance/lang-bench
mkdir -p results
./run.sh | tee "results/run-$(date +%Y%m%d-%H%M%S).log"
```

Tunables (environment variables): `GO_COUNT` (default 8), `RUST_ITERS` (default 100), `RSS_RUNS` (default 5), `REQUESTS` (default 10000).

Unit checks: `cd go && go test ./lb/` and `cd rust && cargo test --release --lib`.
