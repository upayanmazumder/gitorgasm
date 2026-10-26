package lb

import (
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
)

// Workload 1: parse a GitHub `GET /orgs/{org}/repos` style array and
// normalize it into a compact internal record.

type OwnerJSON struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
}

type PermJSON struct {
	Admin bool `json:"admin"`
	Push  bool `json:"push"`
	Pull  bool `json:"pull"`
}

// RepoJSON holds only the fields Gitorgasm reads. The decoder skips the rest.
type RepoJSON struct {
	ID              int64     `json:"id"`
	NodeID          string    `json:"node_id"`
	Name            string    `json:"name"`
	FullName        string    `json:"full_name"`
	Private         bool      `json:"private"`
	Owner           OwnerJSON `json:"owner"`
	Description     *string   `json:"description"`
	Fork            bool      `json:"fork"`
	Archived        bool      `json:"archived"`
	Visibility      string    `json:"visibility"`
	DefaultBranch   string    `json:"default_branch"`
	PushedAt        string    `json:"pushed_at"`
	UpdatedAt       string    `json:"updated_at"`
	StargazersCount int64     `json:"stargazers_count"`
	Language        *string   `json:"language"`
	Topics          []string  `json:"topics"`
	Permissions     *PermJSON `json:"permissions"`
}

const (
	FlagPrivate  uint8 = 1 << 0
	FlagFork     uint8 = 1 << 1
	FlagArchived uint8 = 1 << 2

	PermAdmin uint8 = 1 << 0
	PermPush  uint8 = 1 << 1
	PermPull  uint8 = 1 << 2
)

// RepoRecord is the compact internal form.
type RepoRecord struct {
	ID            int64
	PushedAt      int64 // unix seconds
	UpdatedAt     int64 // unix seconds
	Name          string
	Owner         string
	DefaultBranch string
	Language      string
	Topics        []string
	Stars         int32
	Flags         uint8
	Visibility    uint8 // 0 public, 1 private, 2 internal
	Perm          uint8
}

// ParseRepos decodes with encoding/json (v1) and normalizes.
func ParseRepos(data []byte) ([]RepoRecord, error) {
	var raw []RepoJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return normalize(raw)
}

// ParseReposV2 decodes with encoding/json/v2 (stdlib in Go 1.27) and normalizes.
func ParseReposV2(data []byte) ([]RepoRecord, error) {
	var raw []RepoJSON
	if err := jsonv2.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return normalize(raw)
}

func normalize(raw []RepoJSON) ([]RepoRecord, error) {
	out := make([]RepoRecord, len(raw))
	for i := range raw {
		r := &raw[i]
		var flags uint8
		if r.Private {
			flags |= FlagPrivate
		}
		if r.Fork {
			flags |= FlagFork
		}
		if r.Archived {
			flags |= FlagArchived
		}
		var vis uint8
		switch r.Visibility {
		case "public":
			vis = 0
		case "private":
			vis = 1
		case "internal":
			vis = 2
		default:
			return nil, fmt.Errorf("repo %d: unknown visibility %q", r.ID, r.Visibility)
		}
		var perm uint8
		if p := r.Permissions; p != nil {
			if p.Admin {
				perm |= PermAdmin
			}
			if p.Push {
				perm |= PermPush
			}
			if p.Pull {
				perm |= PermPull
			}
		}
		pushed, err := ParseTimestamp(r.PushedAt)
		if err != nil {
			return nil, err
		}
		updated, err := ParseTimestamp(r.UpdatedAt)
		if err != nil {
			return nil, err
		}
		lang := ""
		if r.Language != nil {
			lang = *r.Language
		}
		out[i] = RepoRecord{
			ID: r.ID, PushedAt: pushed, UpdatedAt: updated,
			Name: r.Name, Owner: r.Owner.Login, DefaultBranch: r.DefaultBranch,
			Language: lang, Topics: r.Topics, Stars: int32(r.StargazersCount),
			Flags: flags, Visibility: vis, Perm: perm,
		}
	}
	return out, nil
}

// RepoChecksum lets Go and Rust prove they produced the same records.
func RepoChecksum(recs []RepoRecord) uint64 {
	var s uint64
	for i := range recs {
		r := &recs[i]
		s += uint64(r.ID) + uint64(r.PushedAt) + uint64(r.UpdatedAt) + uint64(r.Stars)
		s += uint64(r.Flags)<<8 | uint64(r.Visibility)<<4 | uint64(r.Perm)
		s += uint64(len(r.Topics)) + uint64(len(r.Name)) + uint64(len(r.Language))
	}
	return s
}

var errTS = errors.New("bad timestamp")

// ParseTimestamp parses the fixed GitHub layout "YYYY-MM-DDTHH:MM:SSZ".
// Both languages use this hand-written parser so the comparison is fair.
func ParseTimestamp(s string) (int64, error) {
	if len(s) != 20 || s[4] != '-' || s[7] != '-' || s[10] != 'T' || s[13] != ':' || s[16] != ':' || s[19] != 'Z' {
		return 0, errTS
	}
	num := func(a, b int) (int64, error) {
		var v int64
		for i := a; i < b; i++ {
			c := s[i]
			if c < '0' || c > '9' {
				return 0, errTS
			}
			v = v*10 + int64(c-'0')
		}
		return v, nil
	}
	y, e1 := num(0, 4)
	mo, e2 := num(5, 7)
	d, e3 := num(8, 10)
	h, e4 := num(11, 13)
	mi, e5 := num(14, 16)
	se, e6 := num(17, 19)
	if err := errors.Join(e1, e2, e3, e4, e5, e6); err != nil {
		return 0, err
	}
	return daysFromCivil(y, mo, d)*86400 + h*3600 + mi*60 + se, nil
}

// daysFromCivil is Howard Hinnant's algorithm.
func daysFromCivil(y, m, d int64) int64 {
	if m <= 2 {
		y--
	}
	era := y / 400
	if y < 0 && y%400 != 0 {
		era--
	}
	yoe := y - era*400
	mp := (m + 9) % 12
	doy := (153*mp+2)/5 + d - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe - 719468
}
