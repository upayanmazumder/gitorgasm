# Go vs Rust benchmark results

Method: see [README.md](README.md). All numbers below are **Verified**. They come from two full runs of `run.sh` on 2026-10-26. The raw logs are:

- Run A: [`results/run-20261026-191956.log`](results/run-20261026-191956.log)
- Run B: [`results/run-20261026-192141.log`](results/run-20261026-192141.log)

## Environment

| Item            | Value                                                                       | Source                                               |
| --------------- | --------------------------------------------------------------------------- | ---------------------------------------------------- |
| CPU             | AMD Ryzen 7 7840U w/ Radeon(TM) 780M Graphics, 16 logical CPUs              | `/proc/cpuinfo`, `nproc`                             |
| RAM             | 14,100,864 kB (MemTotal)                                                    | `/proc/meminfo`                                      |
| OS              | CachyOS, Linux 7.3.0-5-cachyos, x86_64                                      | `/etc/os-release`, `uname -sr`                       |
| Power           | AC online, governor `performance`, boost on                                 | `/sys/class/power_supply`, `/sys/devices/system/cpu` |
| Go              | go1.27.1 linux/amd64, `GOAMD64` default (v1)                                | `go version`                                         |
| Rust            | rustc 1.97.1, cargo 1.97.1, release profile default (opt-level 3, no LTO)   | `rustc --version`                                    |
| Rust crates     | serde 1.0.229, serde_json 1.0.154, axum 0.8.9, tokio 1.53.2                 | `rust/Cargo.lock`                                    |
| Background load | load average 8.49 to 12.25 during the runs (other jobs on the same machine) | `loadavg_start`/`loadavg_end` in the logs            |
| Fixture         | 9,784,235 bytes, SHA-256 `22afc522…438e1a95`                                | `fixture` section in the logs                        |

**Verified**: Go and Rust produce identical output. Repo checksum `35159376198174` in both. Score top user `977` (93.674433), id checksum `16510352`, score sum `7575.960094` in both (`equivalence` section).

## Results

Time values: Go = median of 8 `ns/op` values from `go test -bench`. Rust = median of 100 timed runs. The range is the min and max of the 8 Go values, or min and p95 of the 100 Rust runs.

| Metric                                                       | Go                                | Rust                                       | Rust / Go        |
| ------------------------------------------------------------ | --------------------------------- | ------------------------------------------ | ---------------- |
| Parse 10k repos, `encoding/json` v1, `GOMAXPROCS=1` (A / B)  | 51.7 / 50.3 ms (range 43.8–70.7)  | —                                          | —                |
| Parse 10k repos, `encoding/json` v1, `GOMAXPROCS=16` (A / B) | 50.5 / 60.4 ms (range 42.3–65.8)  | —                                          | —                |
| Parse 10k repos, `encoding/json/v2`, `GOMAXPROCS=1` (A / B)  | 43.4 / 49.9 ms (range 39.7–58.6)  | —                                          | —                |
| Parse 10k repos, `encoding/json/v2`, `GOMAXPROCS=16` (A / B) | 47.0 / 42.5 ms (range 35.3–70.6)  | —                                          | —                |
| Parse 10k repos, `serde_json` (A / B)                        | —                                 | 20.1 / 24.6 ms (min 15.6, p95 27.5 / 29.9) | about 0.4–0.5×   |
| Parse: allocated bytes / allocations per op                  | 12.9 MB / 113,618                 | 17.4 MB / 130,875                          | —                |
| Score 1M events + top 100, `GOMAXPROCS=1` (A / B)            | 35.0 / 33.1 ms (range 24.2–42.8)  | —                                          | —                |
| Score 1M events + top 100, `GOMAXPROCS=16` (A / B)           | 27.2 / 35.7 ms (range 22.8–43.4)  | —                                          | —                |
| Score 1M events + top 100 (A / B)                            | —                                 | 13.6 / 14.8 ms (min 10.7, p95 19.5 / 23.4) | about 0.4–0.5×   |
| Score: allocated bytes / allocations per op                  | 5.4 MB / 47,704                   | 8.2 MB / 16                                | —                |
| Peak RSS, standalone `repos` (median of 5, A / B)            | 29,740 / 27,828 kB                | 20,372 / 20,384 kB                         | about 0.7×       |
| Peak RSS, standalone `score` (median of 5, A / B)            | 35,216 / 35,172 kB                | 31,624 / 31,612 kB                         | about 0.9×       |
| Binary size, workload `standalone` (default / stripped)      | 4,103,851 / 2,629,792 B (`-s -w`) | 657,224 / 510,576 B (`strip`)              | about 0.16–0.19× |
| Binary size, hello service (default / stripped)              | 8,898,460 / 5,886,215 B (`-s -w`) | 1,668,360 / 1,159,704 B (`strip`)          | about 0.19–0.20× |
| Hello service RSS 1 s after start (A / B)                    | 11,392 / 11,376 kB, 6 threads     | 3,724 / 3,636 kB, 17 threads               | about 0.32×      |
| Hello service RSS after 10k sequential requests (A / B)      | 16,764 / 16,096 kB, 11–12 threads | 4,032 / 3,944 kB, 17 threads               | about 0.24×      |
| 10k sequential keep-alive requests, p50 / p99 latency (A)    | 57 / 150 µs, 15,850 req/s         | 63 / 133 µs, 15,021 req/s                  | —                |
| 10k sequential keep-alive requests, p50 / p99 latency (B)    | 72 / 187 µs, 12,598 req/s         | 39 / 109 µs, 22,310 req/s                  | —                |

Notes on the table:

- The Go `GOMAXPROCS=16` rows let the garbage collector run on other cores. The Rust code is single-threaded. Compare Rust against both Go rows.
- Go `allocs/op` for `Score` is high because the Go code stores `*userState` in the map (one heap object per user). A value-typed map would reduce it. Rust stores the state inline in the `HashMap`.
- The Rust allocation counter counts each `realloc` as one allocation of the new size. Go counts each slice growth as a new allocation. The byte totals are therefore comparable only roughly.
- The `elapsed=` and `VmHWM=` values in the `equivalence` section are single cold runs. They include process start, file read, and (for `score`) event generation. Do not compare them with the benchmark rows.
- The request latency rows are client-bound and noisy. Run A and run B disagree on which server is faster. **Decision**: draw no throughput conclusion from them.
- Tokio started one worker thread per logical CPU (16 + main = 17). On the 4-vCPU target node it would start 4 workers. **Assumption**: idle RSS on the node will then be slightly lower for Rust. This was not measured.

## Failures and skipped items

- No measurement failed.
- `oha`, `hey`, and `wrk` are not installed (`which` returned nothing). **Decision**: a small Go client (`go/cmd/loadclient`) sends the requests instead. Both servers get the same client.
- criterion was not used. A simple harness (`rust/src/bin/bench.rs`) measures Rust. It reports median and p95. Go `testing.B` reports a mean over `b.N` iterations. The statistics are therefore not identical.
- Run A printed a stray `n/a` line after `ac_online=1`. This was a bug in the `run.sh` power check. It was fixed before run B. It does not affect any measurement.

## Decision impact

Conclusions (**Verified** by the table, for this machine and this synthetic data):

1. Rust takes about 40–50% of the Go time for both workloads. Absolute times are small in both languages. Go parses 10,000 repos in about 40–60 ms. Rust needs about 20–25 ms.
2. Rust uses less memory. A hello service uses about 4 MiB in Rust and 11–16 MiB in Go. The workload peak RSS is 10–30% lower in Rust.
3. Rust binaries are about 5× smaller.

What this means for Gitorgasm:

- **Assumption**: the GitHub API returns at most 100 repos per page, and each page takes tens to hundreds of milliseconds over the network. Parse cost per page is then about 0.2 ms (Rust) or 0.5 ms (Go). Network latency and rate limits dominate. CPU speed is not the deciding factor for sync jobs.
- **Assumption**: scoring runs in a background job, not per request. 15–35 ms for 1M events is small in both languages.
- The cluster is memory-constrained (about 79% of memory requests allocated). Rust saves about 8–12 MiB RSS per idle service process, measured here. With 2–4 Gitorgasm backend pods, the total saving is about 20–50 MiB. **Proposal**: treat this as a small advantage, not a deciding one.
- **Proposal**: if Gitorgasm uses Go, set `GOMEMLIMIT` and a memory limit on each pod. Go heap use grows with `GOGC` (default 100) until the GC runs.
- **Proposal**: choose the language on other factors (GitHub App library support, team speed, build time in CI). Both languages meet the performance needs shown here.

Limitations:

- The data is synthetic. The fixture has 27 top-level fields per repo (`go/lb/fixture.go`). **Assumption**: real `GET /orgs/{org}/repos` objects have about 80 top-level fields, mostly URLs, so real parse cost per repo is higher. Real event distributions are unknown.
- One machine only: a laptop with a Zen 4 CPU. The target is a shared Hetzner vCPU. Absolute numbers will differ there. **Assumption**: the ratios will be similar.
- Other jobs ran on the same machine during the runs (load average 8–12). Run-to-run variation is up to about 20% for medians and more for single values. Only differences of about 2× or more are reliable.
- Only two runs were made. The services ran without container limits, TLS, a database, or concurrent clients.
- The Go code uses the standard library only. Third-party Go JSON libraries were not tested. The Rust map uses the default SipHash hasher. A faster hasher was not tested.
