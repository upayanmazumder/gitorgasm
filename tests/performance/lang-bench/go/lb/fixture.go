package lb

import (
	"encoding/json"
	"fmt"
	"time"
)

// Extra fields force decoders to skip unknown GitHub response fields.
type fixtureRepo struct {
	ID              int64           `json:"id"`
	NodeID          string          `json:"node_id"`
	Name            string          `json:"name"`
	FullName        string          `json:"full_name"`
	Private         bool            `json:"private"`
	Owner           fixtureOwner    `json:"owner"`
	HTMLURL         string          `json:"html_url"`
	Description     *string         `json:"description"`
	Fork            bool            `json:"fork"`
	URL             string          `json:"url"`
	CloneURL        string          `json:"clone_url"`
	CreatedAt       string          `json:"created_at"`
	UpdatedAt       string          `json:"updated_at"`
	PushedAt        string          `json:"pushed_at"`
	Size            int64           `json:"size"`
	StargazersCount int64           `json:"stargazers_count"`
	WatchersCount   int64           `json:"watchers_count"`
	Language        *string         `json:"language"`
	ForksCount      int64           `json:"forks_count"`
	OpenIssuesCount int64           `json:"open_issues_count"`
	HasIssues       bool            `json:"has_issues"`
	Archived        bool            `json:"archived"`
	License         *fixtureLicense `json:"license"`
	Topics          []string        `json:"topics"`
	Visibility      string          `json:"visibility"`
	DefaultBranch   string          `json:"default_branch"`
	Permissions     PermJSON        `json:"permissions"`
}

type fixtureOwner struct {
	Login     string `json:"login"`
	ID        int64  `json:"id"`
	NodeID    string `json:"node_id"`
	AvatarURL string `json:"avatar_url"`
	Type      string `json:"type"`
}

type fixtureLicense struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	SPDXID string `json:"spdx_id"`
}

var (
	langs    = []string{"Go", "Rust", "TypeScript", "Python", "JavaScript", "C++", "Java", "Kotlin", "Dart", "Shell"}
	topicSet = []string{"hacktoberfest", "web", "api", "cli", "kubernetes", "nextjs", "react", "ml", "devops", "android", "flutter", "rust", "go", "database", "security", "docs"}
	visSet   = []string{"public", "public", "public", "private", "internal"}
	branches = []string{"main", "main", "main", "master", "dev"}
)

func ts(base time.Time, r *SplitMix64, spanSec uint64) string {
	return base.Add(time.Duration(r.Next()%spanSec) * time.Second).UTC().Format("2006-01-02T15:04:05Z")
}

// GenerateReposJSON returns a deterministic JSON array of n repo objects.
func GenerateReposJSON(n int, seed uint64) ([]byte, error) {
	r := NewRNG(seed)
	base := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	const org, orgID = "example-org", int64(56789012)
	repos := make([]fixtureRepo, n)
	for i := range repos {
		id := int64(100000000 + i*7919)
		name := fmt.Sprintf("project-%05d", i)
		full := org + "/" + name
		var desc *string
		if r.Next()%4 != 0 {
			d := fmt.Sprintf("Synthetic repository %d used for parser benchmarks.", i)
			desc = &d
		}
		var lang *string
		if r.Next()%8 != 0 {
			l := langs[r.Next()%uint64(len(langs))]
			lang = &l
		}
		topics := make([]string, r.Next()%6)
		for t := range topics {
			topics[t] = topicSet[r.Next()%uint64(len(topicSet))]
		}
		var lic *fixtureLicense
		if r.Next()%3 != 0 {
			lic = &fixtureLicense{Key: "mit", Name: "MIT License", SPDXID: "MIT"}
		}
		vis := visSet[r.Next()%uint64(len(visSet))]
		repos[i] = fixtureRepo{
			ID: id, NodeID: fmt.Sprintf("R_kgDO%08X", id), Name: name, FullName: full,
			Private: vis != "public",
			Owner: fixtureOwner{Login: org, ID: orgID, NodeID: "O_kgDOA2Yx5A",
				AvatarURL: "https://avatars.githubusercontent.com/u/56789012?v=4", Type: "Organization"},
			HTMLURL: "https://github.com/" + full, Description: desc,
			Fork: r.Next()%10 == 0, URL: "https://api.github.com/repos/" + full,
			CloneURL:  "https://github.com/" + full + ".git",
			CreatedAt: ts(base, r, 365*86400), UpdatedAt: ts(base, r, 3*365*86400),
			PushedAt: ts(base, r, 3*365*86400), Size: int64(r.Next() % 500000),
			StargazersCount: int64(r.Next() % 5000), WatchersCount: int64(r.Next() % 5000),
			Language: lang, ForksCount: int64(r.Next() % 300), OpenIssuesCount: int64(r.Next() % 120),
			HasIssues: true, Archived: r.Next()%12 == 0, License: lic, Topics: topics,
			Visibility: vis, DefaultBranch: branches[r.Next()%uint64(len(branches))],
			Permissions: PermJSON{Admin: r.Next()%5 == 0, Push: r.Next()%2 == 0, Pull: true},
		}
	}
	return json.Marshal(repos)
}
