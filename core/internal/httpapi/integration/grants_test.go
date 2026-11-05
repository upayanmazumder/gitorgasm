package integration_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/upayanmazumder/gitorgasm/core/internal/config"
	"github.com/upayanmazumder/gitorgasm/core/internal/secure"
)

// fakeGitHub serves the user-token endpoints that decide installation grants.
type fakeGitHub struct {
	mu            sync.Mutex
	installs      string            // JSON array of installations for GET /user/installations
	memberships   map[string]string // org login -> membership JSON; a missing org answers 404
	forbidden     map[string]bool   // org login -> 403
	installsCode  int               // non-zero replaces the GET /user/installations answer
	installsCalls int
	token         string
}

func (f *fakeGitHub) set(fn func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn()
}

func (f *fakeGitHub) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.installsCalls
}

func (f *fakeGitHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if r.URL.Path == "/login/oauth/access_token" {
		_, _ = w.Write([]byte(`{"access_token":"` + f.token + `","token_type":"bearer"}`))
		return
	}
	if r.Header.Get("Authorization") != "Bearer "+f.token {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
		return
	}
	switch {
	case r.URL.Path == "/user":
		_, _ = w.Write([]byte(`{"id":1,"login":"alice"}`))
	case r.URL.Path == "/user/installations":
		f.installsCalls++
		if f.installsCode != 0 {
			if f.installsCode == http.StatusTooManyRequests {
				w.Header().Set("Retry-After", "30")
			}
			w.WriteHeader(f.installsCode)
			_, _ = w.Write([]byte(`{"message":"upstream-private-detail"}`))
			return
		}
		_, _ = w.Write([]byte(`{"total_count":0,"installations":` + f.installs + `}`))
	case strings.HasPrefix(r.URL.Path, "/user/memberships/orgs/"):
		org := strings.TrimPrefix(r.URL.Path, "/user/memberships/orgs/")
		if f.forbidden[org] {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"Resource protected by organization SAML enforcement."}`))
			return
		}
		body, ok := f.memberships[org]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Not Found"}`))
			return
		}
		_, _ = w.Write([]byte(body))
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func membershipJSON(org string, orgID int64, state, role string) string {
	return `{"state":"` + state + `","role":"` + role + `","organization":{"id":` + strconv.FormatInt(orgID, 10) +
		`,"login":"` + org + `"},"user":{"id":1,"login":"alice"}}`
}

func orgInstallation(id int64, org string, orgID int64) string {
	return `{"id":` + strconv.FormatInt(id, 10) + `,"repository_selection":"all","account":{"id":` + strconv.FormatInt(orgID, 10) +
		`,"login":"` + org + `","type":"Organization"}}`
}

func userInstallation(id int64, login string, userID int64) string {
	return `{"id":` + strconv.FormatInt(id, 10) + `,"repository_selection":"all","account":{"id":` + strconv.FormatInt(userID, 10) +
		`,"login":"` + login + `","type":"User"}}`
}

func newGrantHarness(t *testing.T, gh *fakeGitHub) *harness {
	t.Helper()
	upstream := httptest.NewServer(gh)
	t.Cleanup(upstream.Close)
	return newHarnessWithGitHub(t, &config.GitHub{
		APIBaseURL: upstream.URL, WebBaseURL: upstream.URL, InstallURL: upstream.URL + "/install",
		OAuthCallback: appOrigin + "/api/v1/auth/callback", RequestTimeout: time.Second,
	})
}

// sealSessionToken stores a real sealed GitHub token for the session so it can be re-verified.
func (h *harness) sealSessionToken(session, token string, expiresAt *time.Time) {
	h.t.Helper()
	sealer, err := secure.NewSealer(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		h.t.Fatal(err)
	}
	var userID int64
	if err := h.pool.QueryRow(context.Background(), `SELECT user_id FROM sessions WHERE id_hash = $1`, secure.Hash(session)).Scan(&userID); err != nil {
		h.t.Fatal(err)
	}
	sealed := sealer.Seal([]byte(token), []byte(strconv.FormatInt(userID, 10)))
	h.exec(`UPDATE sessions SET github_token_sealed = $2, key_version = $3, github_token_expires_at = $4 WHERE id_hash = $1`,
		secure.Hash(session), sealed, secure.KeyVersion, expiresAt)
}

func (h *harness) ageGrants(session string) {
	h.t.Helper()
	h.exec(`UPDATE sessions SET grants_verified_at = now() - interval '16 minutes' WHERE id_hash = $1`, secure.Hash(session))
}

func (h *harness) grantCount() int {
	h.t.Helper()
	var n int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM user_installations`).Scan(&n); err != nil {
		h.t.Fatal(err)
	}
	return n
}

func TestLoginGrantsOnlyVerifiedMembersAndOwners(t *testing.T) {
	gh := &fakeGitHub{token: "alice-token", forbidden: map[string]bool{"saml-org": true}}
	gh.installs = "[" + strings.Join([]string{
		orgInstallation(10, "member-org", 100),
		orgInstallation(20, "collab-org", 200), // alice only collaborates on one repository
		orgInstallation(30, "pending-org", 300),
		orgInstallation(40, "billing-org", 400),
		orgInstallation(70, "saml-org", 700),
		userInstallation(50, "alice", 1),
		userInstallation(60, "bob", 999), // alice collaborates on a repository owned by bob
	}, ",") + "]"
	gh.memberships = map[string]string{
		"member-org":  membershipJSON("member-org", 100, "active", "member"),
		"pending-org": membershipJSON("pending-org", 300, "pending", "member"),
		"billing-org": membershipJSON("billing-org", 400, "active", "billing_manager"),
	}
	h := newGrantHarness(t, gh)
	h.seedTenant(3, "member-org", 100, 10, "public-tool")
	h.seedTenant(4, "collab-org", 200, 20, "secret-roadmap")
	h.seedTenant(5, "pending-org", 300, 30, "secret-pending")
	h.seedTenant(6, "billing-org", 400, 40, "secret-billing")
	h.seedTenant(7, "saml-org", 700, 70, "secret-saml")
	h.seedTenant(8, "alice", 1, 50, "dotfiles")
	h.seedTenant(9, "bob", 999, 60, "secret-bob")
	h.exec(`DELETE FROM user_installations`)

	cookie, state, _ := startOAuth(t, h, "")
	rec := completeOAuth(t, h, cookie, state, "")
	if rec.Code != http.StatusFound {
		t.Fatalf("callback = %d %s", rec.Code, rec.Body)
	}
	session := responseCookie(t, rec, "__Host-gitorgasm_session").Value

	orgs := decode[page](t, h.do("GET", "/v1/organizations", session, nil, nil))
	if len(orgs.Data) != 2 || orgs.Data[0].Login != "alice" || orgs.Data[1].Login != "member-org" {
		t.Fatalf("granted organizations = %+v, want alice and member-org", orgs.Data)
	}
	for _, login := range []string{"collab-org", "pending-org", "billing-org", "saml-org", "bob"} {
		for _, path := range []string{"/repositories", "/scores", "/scores/presets"} {
			if rec := h.do("GET", "/v1/organizations/"+login+path, session, nil, nil); rec.Code != http.StatusNotFound {
				t.Fatalf("%s%s = %d %s, want 404", login, path, rec.Code, rec.Body)
			}
		}
		if rec := h.do("POST", "/v1/organizations/"+login+"/sync", session, nil, map[string]string{"Origin": appOrigin}); rec.Code != http.StatusNotFound {
			t.Fatalf("%s sync = %d, want 404", login, rec.Code)
		}
	}
	if rec := h.do("GET", "/v1/organizations/member-org/repositories", session, nil, nil); rec.Code != http.StatusOK {
		t.Fatalf("member-org repositories = %d", rec.Code)
	}
	if rec := h.do("GET", "/v1/organizations/alice/repositories", session, nil, nil); rec.Code != http.StatusOK {
		t.Fatalf("own account repositories = %d", rec.Code)
	}
	if h.grantCount() != 2 {
		t.Fatalf("stored grants = %d, want 2", h.grantCount())
	}
}

func TestStaleGrantsAreReverifiedWithinTheDocumentedBound(t *testing.T) {
	gh := &fakeGitHub{token: "alice-token", installs: "[" + orgInstallation(10, "acme", 100) + "]",
		memberships: map[string]string{"acme": membershipJSON("acme", 100, "active", "member")}}
	h := newGrantHarness(t, gh)
	session := h.seedTenant(1, "acme", 100, 10, "repo")
	h.sealSessionToken(session, "alice-token", nil)
	repos := "/v1/organizations/acme/repositories"

	if rec := h.do("GET", repos, session, nil, nil); rec.Code != http.StatusOK || gh.calls() != 0 {
		t.Fatalf("fresh grants = %d with %d GitHub calls, want 200 and none", rec.Code, gh.calls())
	}

	h.ageGrants(session)
	if rec := h.do("GET", repos, session, nil, nil); rec.Code != http.StatusOK || gh.calls() != 1 {
		t.Fatalf("still a member = %d with %d GitHub calls, want 200 and one", rec.Code, gh.calls())
	}
	if rec := h.do("GET", repos, session, nil, nil); rec.Code != http.StatusOK || gh.calls() != 1 {
		t.Fatalf("verified session re-checked too soon: %d calls", gh.calls())
	}

	// Removed from the organization but still a collaborator: GitHub keeps listing the installation.
	gh.set(func() { gh.memberships = map[string]string{} })
	h.ageGrants(session)
	if rec := h.do("GET", repos, session, nil, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("removed member = %d %s, want 404", rec.Code, rec.Body)
	}
	if h.grantCount() != 0 {
		t.Fatalf("grants after removal = %d, want 0", h.grantCount())
	}

	// A user who loses all repository access no longer sees the installation listed at all.
	gh.set(func() {
		gh.memberships = map[string]string{"acme": membershipJSON("acme", 100, "active", "member")}
	})
	h.ageGrants(session)
	if rec := h.do("GET", repos, session, nil, nil); rec.Code != http.StatusOK {
		t.Fatalf("re-added member = %d, want 200", rec.Code)
	}
	gh.set(func() { gh.installs = "[]" })
	h.ageGrants(session)
	if rec := h.do("GET", "/v1/organizations", session, nil, nil); rec.Code != http.StatusOK || len(decode[page](t, rec).Data) != 0 {
		t.Fatalf("organizations after losing access = %d %s", rec.Code, rec.Body)
	}
}

func TestFailedReverificationNeverServesStaleGrants(t *testing.T) {
	gh := &fakeGitHub{token: "alice-token", installs: "[" + orgInstallation(10, "acme", 100) + "]",
		memberships: map[string]string{"acme": membershipJSON("acme", 100, "active", "member")}}
	h := newGrantHarness(t, gh)
	session := h.seedTenant(1, "acme", 100, 10, "repo")
	h.sealSessionToken(session, "alice-token", nil)
	repos := "/v1/organizations/acme/repositories"

	// A user cannot keep access by making GitHub fail or rate limit its own token.
	for _, code := range []int{http.StatusBadGateway, http.StatusTooManyRequests} {
		gh.set(func() { gh.installsCode = code })
		h.ageGrants(session)
		rec := h.do("GET", repos, session, nil, nil)
		if rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), "upstream-private-detail") {
			t.Fatalf("upstream %d = %d %s, want 503 without upstream detail", code, rec.Code, rec.Body)
		}
	}
	if h.grantCount() != 1 {
		t.Fatal("a failed re-verification must not delete grants")
	}
	gh.set(func() { gh.installsCode = 0 })
	if rec := h.do("GET", repos, session, nil, nil); rec.Code != http.StatusOK {
		t.Fatalf("recovered = %d, want 200", rec.Code)
	}

	// A token GitHub rejects ends the session.
	gh.set(func() { gh.token = "rotated" })
	h.ageGrants(session)
	if rec := h.do("GET", repos, session, nil, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token = %d, want 401", rec.Code)
	}
	if rec := h.do("GET", "/v1/me", session, nil, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("session after revoked token = %d, want 401", rec.Code)
	}
}

func TestExpiredUserTokenCannotReverifyGrants(t *testing.T) {
	gh := &fakeGitHub{token: "alice-token", installs: "[" + orgInstallation(10, "acme", 100) + "]",
		memberships: map[string]string{"acme": membershipJSON("acme", 100, "active", "member")}}
	h := newGrantHarness(t, gh)
	session := h.seedTenant(1, "acme", 100, 10, "repo")
	past := time.Now().Add(-time.Minute)
	h.sealSessionToken(session, "alice-token", &past)
	h.ageGrants(session)
	rec := h.do("GET", "/v1/organizations/acme/repositories", session, nil, nil)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "github_reauthentication_required") || gh.calls() != 0 {
		t.Fatalf("expired token = %d %s with %d GitHub calls", rec.Code, rec.Body, gh.calls())
	}
}
