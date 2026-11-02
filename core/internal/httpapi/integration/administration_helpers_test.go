package integration_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/config"
	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
	"github.com/upayanmazumder/gitorgasm/core/internal/secure"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/identity"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/tenancy"
)

const (
	adminInstallation = 10
	adminAccount      = 100
)

// adminGitHub serves the read endpoints that the administration API calls for organization acme.
type adminGitHub struct {
	t   *testing.T
	key *rsa.PrivateKey

	mu              sync.Mutex
	members         string            // installation Members permission
	suspended       bool              // installation suspension
	directoryStatus int               // nonzero forces a directory failure with private upstream text
	roles           map[string]string // user token -> organization role; "revoked" answers 401
	calls           []string          // "METHOD path" of every request
	tokens          []string          // Authorization values of user-authenticated requests
	searchAuth      []string          // Authorization values sent to the search endpoint
	queries         []string          // raw query strings of directory requests
}

func newAdminHarness(t *testing.T) (*harness, *adminGitHub) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &adminGitHub{t: t, key: key, members: "write", roles: map[string]string{}}
	srv := httptest.NewServer(f.handler())
	t.Cleanup(srv.Close)
	h := newHarnessWithGitHub(t, &config.GitHub{AppID: 42, PrivateKey: key, APIBaseURL: srv.URL, WebBaseURL: srv.URL, RequestTimeout: 5 * time.Second})
	return h, f
}

func (f *adminGitHub) set(fn func(*adminGitHub)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(f)
}

func (f *adminGitHub) snapshot() (calls, tokens, searchAuth, queries []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...), append([]string(nil), f.tokens...), append([]string(nil), f.searchAuth...), append([]string(nil), f.queries...)
}

func (f *adminGitHub) handler() http.Handler {
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
	user := func(r *http.Request) string {
		f.tokens = append(f.tokens, r.Header.Get("Authorization"))
		return strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	}
	writeJSON := func(w http.ResponseWriter, status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	next := func(w http.ResponseWriter, r *http.Request, page int) {
		q := r.URL.Query()
		q.Set("page", strconv.Itoa(page+1))
		w.Header().Set("Link", `<http://`+r.Host+r.URL.Path+`?`+q.Encode()+`>; rel="next"`)
	}
	page := func(r *http.Request) int {
		p, err := strconv.Atoi(r.URL.Query().Get("page"))
		if err != nil || p < 1 {
			return 1
		}
		return p
	}
	mux.HandleFunc("GET /app/installations/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !appAuth(r) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		id, accountID, login := adminInstallation, adminAccount, "acme"
		switch r.PathValue("id") {
		case strconv.Itoa(adminInstallation):
		case "20":
			id, accountID, login = 20, 200, "globex"
		default:
			writeJSON(w, http.StatusNotFound, map[string]string{"message": "Not Found"})
			return
		}
		var suspendedAt any
		if f.suspended {
			suspendedAt = "2026-10-01T00:00:00Z"
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id": id, "repository_selection": "all", "suspended_at": suspendedAt,
			"permissions": map[string]string{"metadata": "read", "members": f.members},
			"account":     map[string]any{"id": accountID, "login": login, "type": "Organization"},
		})
	})
	mux.HandleFunc("GET /user/memberships/orgs/{org}", func(w http.ResponseWriter, r *http.Request) {
		role, ok := f.roles[user(r)]
		if role == "revoked" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"message": "Bad credentials private-detail"})
			return
		}
		org := strings.ToLower(r.PathValue("org"))
		if !ok || (org != "acme" && org != "globex") {
			writeJSON(w, http.StatusNotFound, map[string]string{"message": "Not Found"})
			return
		}
		accountID := adminAccount
		if org == "globex" {
			accountID = 200
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"state": "active", "role": role, "organization": map[string]any{"id": accountID, "login": org},
		})
	})
	mux.HandleFunc("GET /search/users", func(w http.ResponseWriter, r *http.Request) {
		f.searchAuth = append(f.searchAuth, r.Header.Get("Authorization"))
		f.queries = append(f.queries, r.URL.RawQuery)
		p := page(r)
		if p == 1 {
			next(w, r, p)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"total_count": 40, "incomplete_results": true,
			"items": []map[string]any{{"id": 1000 + p, "login": fmt.Sprintf("found%d", p), "type": "User", "email": "private@example.com"}},
		})
	})
	mux.HandleFunc("GET /orgs/{org}/members", func(w http.ResponseWriter, r *http.Request) {
		user(r)
		f.queries = append(f.queries, r.URL.RawQuery)
		p := page(r)
		if p < 3 {
			next(w, r, p)
		}
		writeJSON(w, http.StatusOK, []map[string]any{{"id": 2000 + p, "login": fmt.Sprintf("member%d", p), "type": "User", "site_admin": false}})
	})
	mux.HandleFunc("GET /orgs/{org}/teams", func(w http.ResponseWriter, r *http.Request) {
		user(r)
		f.queries = append(f.queries, r.URL.RawQuery)
		p := page(r)
		if p == 1 {
			next(w, r, p)
		}
		writeJSON(w, http.StatusOK, []map[string]any{{"id": 3000 + p, "name": fmt.Sprintf("Team %d", p), "slug": fmt.Sprintf("team-%d", p), "privacy": "secret", "description": "private"}})
	})
	mux.HandleFunc("GET /orgs/{org}/teams/{slug}", func(w http.ResponseWriter, r *http.Request) {
		user(r)
		if r.PathValue("slug") != "core" {
			writeJSON(w, http.StatusNotFound, map[string]string{"message": "Not Found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": 3000, "name": "Core", "slug": "core"})
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.calls = append(f.calls, r.Method+" "+r.URL.Path)
		if r.Header.Get("X-GitHub-Api-Version") != github.APIVersion {
			f.t.Errorf("%s %s without the pinned API version", r.Method, r.URL.Path)
		}
		if f.directoryStatus != 0 && (r.URL.Path == "/search/users" || strings.HasPrefix(r.URL.Path, "/orgs/")) {
			writeJSON(w, f.directoryStatus, map[string]string{"message": "private-detail token-secret private@example.com"})
			return
		}
		mux.ServeHTTP(w, r)
	})
}

// seedActor grants a user access to acme and returns a session whose sealed token is token.
func (h *harness) seedActor(githubID int64, login, token string) string {
	h.t.Helper()
	ctx := context.Background()
	inst := &models.Installation{ID: adminInstallation, RepositorySelection: "all", Account: &models.Account{ID: adminAccount, Login: "acme", Type: "Organization"}}
	if err := tenancy.UpsertInstallation(ctx, h.pool, inst); err != nil {
		h.t.Fatal(err)
	}
	userID, err := identity.UpsertUser(ctx, h.pool, &models.User{ID: githubID, Login: login})
	if err != nil {
		h.t.Fatal(err)
	}
	if _, err := h.pool.Exec(ctx, `INSERT INTO user_installations (user_id, installation_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, adminInstallation); err != nil {
		h.t.Fatal(err)
	}
	return h.addSession(userID, token)
}

// addSession creates another session of an existing user with its own sealed token.
func (h *harness) addSession(userID int64, token string) string {
	h.t.Helper()
	sealer, err := secure.NewSealer(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		h.t.Fatal(err)
	}
	session := secure.RandomToken()
	if err := identity.CreateSession(context.Background(), h.pool, identity.NewSession{
		IDHash: secure.Hash(session), UserID: userID, KeyVersion: secure.KeyVersion,
		TokenSealed: sealer.Seal([]byte(token), []byte(strconv.FormatInt(userID, 10))), ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		h.t.Fatal(err)
	}
	return session
}

func (h *harness) userID(githubID int64) int64 {
	h.t.Helper()
	var id int64
	if err := h.pool.QueryRow(context.Background(), `SELECT id FROM users WHERE github_id = $1`, githubID).Scan(&id); err != nil {
		h.t.Fatal(err)
	}
	return id
}

type adminError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type adminResult struct {
	Login   string  `json:"login"`
	Status  string  `json:"status"`
	Code    *string `json:"code"`
	Message *string `json:"message"`
}

type adminOperation struct {
	ID          string        `json:"id"`
	Kind        string        `json:"kind"`
	Status      string        `json:"status"`
	TeamSlug    *string       `json:"team_slug"`
	Role        *string       `json:"role"`
	TargetCount int           `json:"target_count"`
	CompletedAt *time.Time    `json:"completed_at"`
	Results     []adminResult `json:"results"`
}

func adminHeaders(key string) map[string]string {
	return map[string]string{"Origin": appOrigin, "Idempotency-Key": key, "Content-Type": "application/json"}
}
