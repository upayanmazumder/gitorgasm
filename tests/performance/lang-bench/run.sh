#!/usr/bin/env bash
# Runs the full Go vs Rust benchmark and prints a raw log to stdout.
# Usage: ./run.sh | tee results/run-$(date +%Y%m%d-%H%M%S).log
set -euo pipefail
cd "$(dirname "$0")"
ROOT=$PWD
GO_COUNT=${GO_COUNT:-8}
RUST_ITERS=${RUST_ITERS:-100}
RSS_RUNS=${RSS_RUNS:-5}
REQUESTS=${REQUESTS:-10000}

section() { printf '\n=== %s ===\n' "$1"; }

section environment
date -u +%Y-%m-%dT%H:%M:%SZ
grep -m1 'model name' /proc/cpuinfo
echo "logical_cpus=$(nproc)"
uname -sr
grep PRETTY_NAME /etc/os-release || true
grep MemTotal /proc/meminfo
echo "governor=$(cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_governor 2>/dev/null || echo n/a)"
echo "ac_online=$( (cat /sys/class/power_supply/*/online 2>/dev/null || true) | head -n1)"
echo "loadavg_start=$(cut -d' ' -f1-3 /proc/loadavg)"
go version
rustc --version
cargo --version

section build
mkdir -p fixtures go/bin
(cd go && go build -o bin/standalone ./cmd/standalone \
  && go build -o bin/hello ./cmd/hello \
  && go build -ldflags='-s -w' -o bin/standalone-sw ./cmd/standalone \
  && go build -ldflags='-s -w' -o bin/hello-sw ./cmd/hello \
  && go build -o bin/loadclient ./cmd/loadclient)
(cd rust && cargo build --release --offline --quiet)
for b in standalone hello; do
  strip -o "rust/target/release/$b-stripped" "rust/target/release/$b"
done

section binary-size-bytes
stat -c '%s %n' go/bin/standalone go/bin/standalone-sw go/bin/hello go/bin/hello-sw \
  rust/target/release/standalone rust/target/release/standalone-stripped \
  rust/target/release/hello rust/target/release/hello-stripped

section fixture
go/bin/standalone gen fixtures/repos.json
sha256sum fixtures/repos.json

section equivalence
go/bin/standalone repos fixtures/repos.json
rust/target/release/standalone repos fixtures/repos.json
go/bin/standalone score
rust/target/release/standalone score

section go-bench
(cd go && go test ./lb/ -run '^$' -bench . -benchmem -cpu 1,16 -count "$GO_COUNT")

section rust-bench
rust/target/release/bench fixtures/repos.json "$RUST_ITERS"

section peak-rss-kb
# /usr/bin/time -v "Maximum resident set size" in kB, RSS_RUNS runs each.
peak() {
  local name=$1; shift
  local vals=()
  for _ in $(seq "$RSS_RUNS"); do
    vals+=("$(/usr/bin/time -v "$@" 2>&1 >/dev/null | awk '/Maximum resident/ {print $6}')")
  done
  echo "$name ${vals[*]}"
}
peak go-repos go/bin/standalone repos fixtures/repos.json
peak rust-repos rust/target/release/standalone repos fixtures/repos.json
peak go-score go/bin/standalone score
peak rust-score rust/target/release/standalone score

section hello-rss-kb
status() { awk '/^(VmRSS|VmHWM|Threads):/ {sub(":", "", $1); printf "%s=%s ", $1, $2} END {print ""}' "/proc/$1/status"; }
hello() {
  local name=$1 addr=$2; shift 2
  "$@" "$addr" 2>/dev/null &
  local pid=$!
  sleep 1
  echo "$name after-startup: $(status $pid)"
  go/bin/loadclient "http://$addr/" "$REQUESTS"
  sleep 1
  echo "$name after-$REQUESTS-requests: $(status $pid)"
  kill "$pid"
  wait "$pid" 2>/dev/null || true
}
hello go-nethttp 127.0.0.1:18080 go/bin/hello
hello rust-axum 127.0.0.1:18081 rust/target/release/hello

section done
echo "loadavg_end=$(cut -d' ' -f1-3 /proc/loadavg)"
