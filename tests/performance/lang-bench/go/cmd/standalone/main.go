// Command standalone runs one workload once, for peak-RSS measurement.
//
//	standalone gen <out.json>   write the 10k-repo fixture
//	standalone repos <in.json>  parse + normalize the fixture (encoding/json v1)
//	standalone score            generate 1M events, score, top-100
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gitorgasm/langbench/lb"
)

func main() {
	if len(os.Args) < 2 {
		fail("usage: standalone gen|repos|score [path]")
	}
	start := time.Now()
	switch os.Args[1] {
	case "gen":
		data, err := lb.GenerateReposJSON(10_000, 42)
		check(err)
		check(os.WriteFile(arg(2), data, 0o644))
		fmt.Printf("wrote %d bytes\n", len(data))
	case "repos":
		data, err := os.ReadFile(arg(2))
		check(err)
		recs, err := lb.ParseRepos(data)
		check(err)
		fmt.Printf("repos=%d checksum=%d\n", len(recs), lb.RepoChecksum(recs))
	case "score":
		events := lb.GenerateEvents(1_000_000, 7)
		top := lb.Score(events, lb.BaseTS+lb.SpanSec)
		ids, sum := lb.ScoreChecksum(top)
		fmt.Printf("top=%d first=%d:%.6f id_checksum=%d score_sum=%.6f\n",
			len(top), top[0].UserID, top[0].Score, ids, sum)
	default:
		fail("unknown mode " + os.Args[1])
	}
	fmt.Printf("elapsed=%s %s\n", time.Since(start), vmHWM())
}

func vmHWM() string {
	b, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return "VmHWM=?"
	}
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, "VmHWM:") {
			return "VmHWM=" + strings.Join(strings.Fields(l)[1:], "")
		}
	}
	return "VmHWM=?"
}

func arg(i int) string {
	if len(os.Args) <= i {
		fail("missing path argument")
	}
	return os.Args[i]
}

func check(err error) {
	if err != nil {
		fail(err.Error())
	}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
