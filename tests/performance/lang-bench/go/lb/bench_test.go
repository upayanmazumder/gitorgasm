package lb

import (
	"os"
	"testing"
)

const (
	RepoCount  = 10_000
	RepoSeed   = 42
	EventCount = 1_000_000
	EventSeed  = 7
)

// fixture loads ../../fixtures/repos.json if present (the same bytes the
// Rust harness reads). Otherwise it generates the fixture in memory.
func fixture(b *testing.B) []byte {
	if data, err := os.ReadFile("../../fixtures/repos.json"); err == nil {
		return data
	}
	data, err := GenerateReposJSON(RepoCount, RepoSeed)
	if err != nil {
		b.Fatal(err)
	}
	return data
}

func benchParse(b *testing.B, parse func([]byte) ([]RepoRecord, error)) {
	data := fixture(b)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	var sum uint64
	for b.Loop() {
		recs, err := parse(data)
		if err != nil {
			b.Fatal(err)
		}
		sum = RepoChecksum(recs)
	}
	b.ReportMetric(float64(sum%1_000_000), "checksum_mod1e6")
}

func BenchmarkParseReposJSONv1(b *testing.B) { benchParse(b, ParseRepos) }
func BenchmarkParseReposJSONv2(b *testing.B) { benchParse(b, ParseReposV2) }

func BenchmarkScore(b *testing.B) {
	events := GenerateEvents(EventCount, EventSeed)
	b.ReportAllocs()
	var top []UserScore
	for b.Loop() {
		top = Score(events, BaseTS+SpanSec)
	}
	if len(top) != TopN {
		b.Fatalf("top len %d", len(top))
	}
}

func TestTimestamp(t *testing.T) {
	got, err := ParseTimestamp("2026-01-01T00:00:00Z")
	if err != nil || got != BaseTS {
		t.Fatalf("got %d %v", got, err)
	}
}

func TestParsersAgree(t *testing.T) {
	data, err := GenerateReposJSON(500, RepoSeed)
	if err != nil {
		t.Fatal(err)
	}
	a, err := ParseRepos(data)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ParseReposV2(data)
	if err != nil {
		t.Fatal(err)
	}
	if RepoChecksum(a) != RepoChecksum(b) {
		t.Fatal("v1 and v2 checksums differ")
	}
}
