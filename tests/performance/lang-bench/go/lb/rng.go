// Package lb holds the Go side of the Gitorgasm language benchmark.
// The Rust crate in ../rust implements the same logic. Keep both in sync.
package lb

// SplitMix64 is a tiny deterministic PRNG. The Rust side uses the same
// algorithm, so both languages see identical synthetic data.
type SplitMix64 struct{ state uint64 }

func NewRNG(seed uint64) *SplitMix64 { return &SplitMix64{state: seed} }

func (r *SplitMix64) Next() uint64 {
	r.state += 0x9E3779B97F4A7C15
	z := r.state
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}
