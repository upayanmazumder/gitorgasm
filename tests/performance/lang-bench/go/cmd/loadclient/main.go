// Command loadclient sends N sequential GET requests over one keep-alive
// connection and prints latency percentiles. No curl, hey, or oha needed.
//
//	loadclient <url> <n>
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strconv"
	"time"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: loadclient <url> <n>")
		os.Exit(1)
	}
	url := os.Args[1]
	n, err := strconv.Atoi(os.Args[2])
	if err != nil || n <= 0 {
		fmt.Fprintln(os.Stderr, "bad n")
		os.Exit(1)
	}
	client := &http.Client{Transport: &http.Transport{MaxIdleConnsPerHost: 1}}
	lat := make([]time.Duration, n)
	start := time.Now()
	for i := range n {
		t := time.Now()
		resp, err := client.Get(url)
		if err != nil {
			fmt.Fprintf(os.Stderr, "request %d: %v\n", i, err)
			os.Exit(1)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			fmt.Fprintf(os.Stderr, "request %d: status %d\n", i, resp.StatusCode)
			os.Exit(1)
		}
		lat[i] = time.Since(t)
	}
	total := time.Since(start)
	slices.Sort(lat)
	pct := func(p float64) time.Duration { return lat[int(p*float64(n-1))] }
	fmt.Printf("requests=%d total=%s rps=%.0f p50=%s p95=%s p99=%s max=%s\n",
		n, total, float64(n)/total.Seconds(), pct(0.50), pct(0.95), pct(0.99), lat[n-1])
}
