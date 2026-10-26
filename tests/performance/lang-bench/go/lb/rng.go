// Package lb implements the Go side of the shared language benchmark.
package lb

// SplitMix64 matches Rust's PRNG so both workloads use identical input.
type SplitMix64 struct{ state uint64 }

func NewRNG(seed uint64) *SplitMix64 { return &SplitMix64{state: seed} }

func (r *SplitMix64) Next() uint64 {
	r.state += 0x9E3779B97F4A7C15
	z := r.state
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}
