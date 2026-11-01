package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/upayanmazumder/gitorgasm/core/internal/config"
)

func TestInstallationReturnRefreshesOnlyGitHubGrants(t *testing.T) {
	var verifier string
	var emptyGrants atomic.Bool
	var failSecondPage atomic.Bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/login/oauth/access_token":
			if err := r.ParseForm(); err != nil || r.Form.Get("code_verifier") != verifier || r.Form.Get("code") != "valid-code" || r.Form.Get("redirect_uri") != appOrigin+"/api/v1/auth/callback" {
				t.Error("token exchange lost PKCE verifier, code, or callback")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"private-user-token","token_type":"bearer"}`))
		case "/user":
			if r.Header.Get("Authorization") != "Bearer private-user-token" {
				t.Error("identity did not use the fresh user token")
			}
			_, _ = w.Write([]byte(`{"id":1,"login":"alice"}`))
		case "/user/installations":
			if r.Header.Get("Authorization") != "Bearer private-user-token" {
				t.Error("installations did not use the fresh user token")
			}
			if emptyGrants.Load() {
				_, _ = w.Write([]byte(`{"installations":[]}`))
				return
			}
			if r.URL.Query().Get("page") == "2" {
				if failSecondPage.Load() {
					w.WriteHeader(http.StatusBadGateway)
					_, _ = w.Write([]byte(`{"message":"private-upstream-detail"}`))
					return
				}
				_, _ = w.Write([]byte(`{"installations":[{"id":40,"repository_selection":"all","account":{"id":400,"login":"second","type":"Organization"}}]}`))
				return
			}
			w.Header().Set("Link", "<http://"+r.Host+"/user/installations?page=2>; rel=\"next\"")
			_, _ = w.Write([]byte(`{"installations":[{"id":30,"repository_selection":"all","account":{"id":300,"login":"fresh","type":"Organization"}}]}`))
		default:
			t.Errorf("unexpected GitHub call %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()
	installURL := upstream.URL + "/apps/test-app/installations/new"
	h := newHarnessWithGitHub(t, &config.GitHub{
		APIBaseURL: upstream.URL, WebBaseURL: upstream.URL, InstallURL: installURL,
		OAuthCallback: appOrigin + "/api/v1/auth/callback", RequestTimeout: time.Second,
	})
	oldSession := h.seedTenant(1, "old", 100, 10)
	victimSession := h.seedTenant(2, "victim", 200, 20, "secret")
	if rec := h.do("GET", "/v1/auth/install", oldSession, nil, nil); rec.Code != http.StatusFound || rec.Header().Get("Location") != installURL {
		t.Fatalf("install = %d %s", rec.Code, rec.Header().Get("Location"))
	}
	cookie, state, gotVerifier := startOAuth(t, h, oldSession)
	verifier = gotVerifier

	// A failure on a later page must not replace grants with a partial list or create a session/job.
	failSecondPage.Store(true)
	rec := completeOAuth(t, h, cookie, state, oldSession)
	if rec.Code != http.StatusBadGateway || strings.Contains(rec.Body.String(), "private-upstream-detail") {
		t.Fatalf("failed refresh = %d %s", rec.Code, rec.Body)
	}
	if rec := h.do("GET", "/v1/organizations/old/repositories", oldSession, nil, nil); rec.Code != http.StatusOK {
		t.Fatal("failed refresh revoked the existing grant")
	}
	assertAuthRows(t, h, 2, 2, 0)
	failSecondPage.Store(false)

	// A late database error must roll back grants, session creation, and transactional jobs together.
	h.exec(`ALTER TABLE audit_events ADD CONSTRAINT reject_test_login CHECK (action <> 'session.created')`)
	cookie, state, verifier = startOAuth(t, h, oldSession)
	rec = completeOAuth(t, h, cookie, state, oldSession)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("late transaction failure = %d %s", rec.Code, rec.Body)
	}
	assertAuthRows(t, h, 2, 2, 0)
	h.exec(`ALTER TABLE audit_events DROP CONSTRAINT reject_test_login`)

	cookie, state, verifier = startOAuth(t, h, oldSession)
	rec = completeOAuth(t, h, cookie, state, oldSession)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != appOrigin+"/organizations" {
		t.Fatalf("successful callback = %d %s", rec.Code, rec.Header().Get("Location"))
	}
	newCookie := responseCookie(t, rec, "__Host-gitorgasm_session")
	if newCookie.Value == oldSession || !newCookie.HttpOnly || !newCookie.Secure || newCookie.SameSite != http.SameSiteLaxMode || newCookie.Path != "/" {
		t.Fatal("callback did not issue a fresh protected session")
	}
	if responseCookie(t, rec, oauthCookieName).MaxAge != -1 {
		t.Fatal("callback did not consume OAuth cookie")
	}
	assertAuthRows(t, h, 3, 3, 2)
	if rec := h.do("GET", "/v1/me", newCookie.Value, nil, nil); rec.Code != http.StatusOK {
		t.Fatalf("new session = %d %s", rec.Code, rec.Body)
	}
	orgs := decode[page](t, h.do("GET", "/v1/organizations", newCookie.Value, nil, nil))
	if len(orgs.Data) != 2 || orgs.Data[0].Login != "fresh" || orgs.Data[1].Login != "second" {
		t.Fatalf("fresh grants = %+v", orgs.Data)
	}
	for _, org := range []string{"old", "victim"} {
		if rec := h.do("GET", "/v1/organizations/"+org+"/repositories", newCookie.Value, nil, nil); rec.Code != http.StatusNotFound {
			t.Fatalf("ungranted %s = %d", org, rec.Code)
		}
	}
	if rec := h.do("GET", "/v1/organizations/victim/repositories", victimSession, nil, nil); rec.Code != http.StatusOK {
		t.Fatal("refresh affected another user's grants")
	}

	// Refreshing with no installations is authoritative, including for already-issued sessions.
	emptyGrants.Store(true)
	cookie, state, verifier = startOAuth(t, h, newCookie.Value)
	rec = completeOAuth(t, h, cookie, state, newCookie.Value)
	if rec.Code != http.StatusFound {
		t.Fatalf("empty grant refresh = %d %s", rec.Code, rec.Body)
	}
	for _, session := range []string{oldSession, newCookie.Value, responseCookie(t, rec, "__Host-gitorgasm_session").Value} {
		if rec := h.do("GET", "/v1/organizations/fresh/repositories", session, nil, nil); rec.Code != http.StatusNotFound {
			t.Fatalf("revoked grant = %d", rec.Code)
		}
	}
}
