package lb

import "sort"

const (
	NumKinds = 10
	NumUsers = 50_000
	NumRepos = 10_000
	// Events span 180 days. Only the last 90 days count.
	SpanSec   = 180 * 86400
	WindowSec = 90 * 86400
	BaseTS    = int64(1_767_225_600) // 2026-01-01T00:00:00Z
	TopN      = 100
)

// Kinds: commit, PR opened/merged, review, issue opened/closed, comment, release, star, discussion.
var Weights = [NumKinds]float64{1, 3, 5, 4, 2, 2, 0.5, 6, 0.2, 1}

type Event struct {
	TS     int64
	UserID uint32
	RepoID uint32
	Kind   uint8
}

// GenerateEvents skews users toward low IDs to model uneven activity.
func GenerateEvents(n int, seed uint64) []Event {
	r := NewRNG(seed)
	ev := make([]Event, n)
	for i := range ev {
		a, b := r.Next()%NumUsers, r.Next()%NumUsers
		ev[i] = Event{
			UserID: uint32(min(a, b)),
			RepoID: uint32(r.Next() % NumRepos),
			Kind:   uint8(r.Next() % NumKinds),
			TS:     BaseTS + int64(r.Next()%SpanSec),
		}
	}
	return ev
}

type userState struct {
	counts [NumKinds]uint32
	score  float64
}

type UserScore struct {
	UserID uint32
	Score  float64
}

// Score applies a 90-day window and diminishing returns per user and event kind.
func Score(events []Event, now int64) []UserScore {
	since := now - WindowSec
	users := make(map[uint32]*userState)
	for i := range events {
		e := &events[i]
		if e.TS < since || e.TS > now {
			continue
		}
		s := users[e.UserID]
		if s == nil {
			s = &userState{}
			users[e.UserID] = s
		}
		n := s.counts[e.Kind]
		s.score += Weights[e.Kind] / (1 + 0.1*float64(n))
		s.counts[e.Kind] = n + 1
	}
	all := make([]UserScore, 0, len(users))
	for id, s := range users {
		all = append(all, UserScore{UserID: id, Score: s.score})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Score != all[j].Score {
			return all[i].Score > all[j].Score
		}
		return all[i].UserID < all[j].UserID
	})
	if len(all) > TopN {
		all = all[:TopN]
	}
	return all
}

// ScoreChecksum lets Go and Rust prove they produced the same top list.
func ScoreChecksum(top []UserScore) (uint64, float64) {
	var ids uint64
	var sum float64
	for i, u := range top {
		ids += uint64(u.UserID) * uint64(i+1)
		sum += u.Score
	}
	return ids, sum
}
