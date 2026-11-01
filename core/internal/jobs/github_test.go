package jobs_test

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/github"
)

const instID = 10

// fakeGitHub serves the endpoints used by a sync. It checks App JWTs and installation tokens.
type fakeGitHub struct {
	key           *rsa.PrivateKey
	rateLimitPage int // when > 0, this repository page answers with a primary rate limit
	gone          bool
	tokenMints    atomic.Int32
	contributions bool
	failEvents    atomic.Bool
	mu            sync.Mutex
	repoStatus    map[string]int // per repository commits status override, for example 404 or 403
	commitCalls   map[string]int
}

func (f *fakeGitHub) handler(t *testing.T) http.Handler {
	mux := http.NewServeMux()
	appAuth := func(r *http.Request) bool {
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		tok, err := jwt.ParseWithClaims(raw, &jwt.RegisteredClaims{}, func(*jwt.Token) (any, error) { return &f.key.PublicKey, nil },
			jwt.WithValidMethods([]string{"RS256"}))
		if err != nil {
			return false
		}
		iss, _ := tok.Claims.GetIssuer()
		return iss == "42"
	}
	mux.HandleFunc("GET /app/installations/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !appAuth(r) {
			w.WriteHeader(401)
			return
		}
		if f.gone {
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"message":"Not Found"}`))
			return
		}
		_, _ = fmt.Fprintf(w, `{"id":%s,"repository_selection":"all","permissions":{"metadata":"read"},
			"account":{"id":100,"login":"acme","type":"Organization","avatar_url":"https://a.example/acme"}}`, r.PathValue("id"))
	})
	mux.HandleFunc("POST /app/installations/{id}/access_tokens", func(w http.ResponseWriter, r *http.Request) {
		if !appAuth(r) {
			w.WriteHeader(401)
			return
		}
		f.tokenMints.Add(1)
		_, _ = fmt.Fprintf(w, `{"token":"ghs_test","expires_at":%q}`, time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
	})
	mux.HandleFunc("GET /installation/repositories", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ghs_test" || r.Header.Get("X-GitHub-Api-Version") != github.APIVersion {
			w.WriteHeader(401)
			return
		}
		pageNo, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if pageNo == 0 {
			pageNo = 1
		}
		if pageNo == f.rateLimitPage {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(2*time.Minute).Unix(), 10))
			w.WriteHeader(403)
			_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
			return
		}
		pages := map[int][]string{1: {"api", "web"}, 2: {"infra"}}
		var repos []map[string]any
		for i, n := range pages[pageNo] {
			repos = append(repos, map[string]any{
				"id": pageNo*100 + i, "name": n, "full_name": "acme/" + n, "owner": map[string]any{"id": 100, "login": "acme"},
				"private": true, "fork": false, "archived": false, "visibility": "private", "default_branch": "main",
				"stargazers_count": 3, "pushed_at": "2026-10-01T00:00:00Z", "updated_at": "2026-10-02T00:00:00Z",
			})
		}
		if pageNo == 1 {
			next := "http://" + r.Host + "/installation/repositories?per_page=100&page=2"
			w.Header().Set("Link", `<`+next+`>; rel="next", <`+next+`>; rel="last"`)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"total_count": 3, "repository_selection": "all", "repositories": repos})
	})
	if f.contributions {
		mux.HandleFunc("GET /repos/{owner}/{repo}/commits", func(w http.ResponseWriter, r *http.Request) {
			repo := r.PathValue("repo")
			f.mu.Lock()
			if f.commitCalls == nil {
				f.commitCalls = map[string]int{}
			}
			f.commitCalls[repo]++
			status := f.repoStatus[repo]
			f.mu.Unlock()
			switch {
			case f.failEvents.Load():
				w.WriteHeader(http.StatusBadGateway)
				return
			case status == http.StatusForbidden:
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Minute).Unix(), 10))
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
				return
			case status != 0:
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"message":"Not Found"}`))
				return
			}
			now := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"sha":    repo + "-sha",
				"author": map[string]any{"id": 7, "login": "alice", "type": "User"},
				"commit": map[string]any{"committer": map[string]any{"date": now}},
			}})
		})
		mux.HandleFunc("GET /repos/{owner}/{repo}/pulls", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("[]"))
		})
		mux.HandleFunc("GET /repos/{owner}/{repo}/issues", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("[]"))
		})
	}
	return mux
}

func (f *fakeGitHub) setRepoStatus(repo string, status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.repoStatus == nil {
		f.repoStatus = map[string]int{}
	}
	f.repoStatus[repo] = status
}

func (f *fakeGitHub) calls(repo string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.commitCalls[repo]
}
