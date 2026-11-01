package httpapi_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/upayanmazumder/gitorgasm/core/internal/config"
)

const oauthCookieName = "__Host-gitorgasm_oauth"

func responseCookie(t *testing.T, rec *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("response did not set %s", name)
	return nil
}

func startOAuth(t *testing.T, h *harness, session string) (*http.Cookie, string, string) {
	t.Helper()
	// A setup return is untrusted. Neither its installation id nor a redirect target is authorization.
	rec := h.do("GET", "/v1/auth/login?installation_id=20&setup_action=install&returnTo=https://evil.example", session, nil, nil)
	if rec.Code != http.StatusFound {
		t.Fatalf("login = %d %s", rec.Code, rec.Body)
	}
	cookie := responseCookie(t, rec, oauthCookieName)
	state, verifier, ok := strings.Cut(cookie.Value, ".")
	if !ok || state == "" || verifier == "" || !cookie.HttpOnly || !cookie.Secure || cookie.Path != "/" || cookie.SameSite != http.SameSiteLaxMode || cookie.MaxAge != 600 {
		t.Fatalf("invalid OAuth cookie attributes: %+v", cookie)
	}
	location, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	query := location.Query()
	sum := sha256.Sum256([]byte(verifier))
	if location.Path != "/login/oauth/authorize" || query.Get("state") != state || query.Get("code_challenge") != base64.RawURLEncoding.EncodeToString(sum[:]) || query.Get("code_challenge_method") != "S256" || query.Get("redirect_uri") != appOrigin+"/api/v1/auth/callback" {
		t.Fatalf("invalid authorization redirect: %s", location)
	}
	if query.Has("installation_id") || query.Has("returnTo") || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("setup query propagated or OAuth response was cacheable")
	}
	return cookie, state, verifier
}

func completeOAuth(t *testing.T, h *harness, cookie *http.Cookie, state, session string) *httptest.ResponseRecorder {
	t.Helper()
	return h.do("GET", "/v1/auth/callback?code=valid-code&state="+url.QueryEscape(state)+"&installation_id=20&returnTo=https://evil.example", session, nil, map[string]string{"Cookie": cookie.String()})
}

func TestOAuthRejectsUntrustedCallbacks(t *testing.T) {
	var calls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upstream.Close()
	h := newHarnessWithGitHub(t, &config.GitHub{APIBaseURL: upstream.URL, WebBaseURL: upstream.URL, RequestTimeout: time.Second})
	cases := []struct {
		name, query, cookie, code string
	}{
		{"missing cookie", "state=expected&code=valid", "", "oauth_state_missing"},
		{"missing state", "code=valid", "expected.verifier", "oauth_state_mismatch"},
		{"mismatched state", "state=other&code=valid", "expected.verifier", "oauth_state_mismatch"},
		{"malformed cookie", "state=expected&code=valid", "expected", "oauth_state_mismatch"},
		{"empty verifier", "state=expected&code=valid", "expected.", "oauth_state_mismatch"},
		{"missing code", "state=expected", "expected.verifier", "oauth_denied"},
		{"denied", "state=expected&error=access_denied&error_description=private-detail", "expected.verifier", "oauth_denied"},
		{"denied with code", "state=expected&error=access_denied&code=valid", "expected.verifier", "oauth_denied"},
		{"forged installation", "installation_id=20&setup_action=install", "", "oauth_state_missing"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			headers := map[string]string{}
			if tc.cookie != "" {
				headers["Cookie"] = oauthCookieName + "=" + tc.cookie
			}
			rec := h.do("GET", "/v1/auth/callback?"+tc.query, "", nil, headers)
			var body struct {
				Error struct{ Code string } `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if rec.Code != http.StatusBadRequest || body.Error.Code != tc.code || rec.Header().Get("Location") != "" || strings.Contains(rec.Body.String(), "private-detail") {
				t.Fatalf("callback = %d %s", rec.Code, rec.Body)
			}
			if responseCookie(t, rec, oauthCookieName).MaxAge != -1 {
				t.Fatal("OAuth cookie not consumed on failure")
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatalf("untrusted callbacks made %d GitHub requests", calls.Load())
	}
	var sessions int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM sessions`).Scan(&sessions); err != nil || sessions != 0 {
		t.Fatalf("untrusted callbacks created sessions: %d (%v)", sessions, err)
	}
}

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

func assertAuthRows(t *testing.T, h *harness, sessions, grants, jobs int) {
	t.Helper()
	var gotSessions, gotGrants, gotJobs int
	err := h.pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM sessions), (SELECT count(*) FROM user_installations), (SELECT count(*) FROM river_job WHERE kind = 'sync_installation')`).Scan(&gotSessions, &gotGrants, &gotJobs)
	if err != nil || gotSessions != sessions || gotGrants != grants || gotJobs != jobs {
		t.Fatalf("sessions/grants/jobs = %d/%d/%d (%v), want %d/%d/%d", gotSessions, gotGrants, gotJobs, err, sessions, grants, jobs)
	}
}

func TestLogoutRevokesOnlyCurrentSessionAndPendingOAuth(t *testing.T) {
	h := newHarness(t)
	session := h.seedTenant(1, "acme", 100, 10)
	otherSession := h.seedTenant(2, "victim", 200, 20)
	if rec := h.do("POST", "/v1/auth/logout", session, nil, map[string]string{"Origin": "https://evil.example"}); rec.Code != http.StatusForbidden {
		t.Fatalf("cross-origin logout = %d", rec.Code)
	}
	if rec := h.do("GET", "/v1/me", session, nil, nil); rec.Code != http.StatusOK {
		t.Fatal("rejected logout revoked session")
	}
	rec := h.do("POST", "/v1/auth/logout", session, nil, map[string]string{
		"Origin": appOrigin,
		"Cookie": "__Host-gitorgasm_session=" + session + "; " + oauthCookieName + "=pending.verifier",
	})
	if rec.Code != http.StatusNoContent || responseCookie(t, rec, "__Host-gitorgasm_session").MaxAge != -1 || responseCookie(t, rec, oauthCookieName).MaxAge != -1 {
		t.Fatalf("logout did not clear browser authentication: %d", rec.Code)
	}
	if rec := h.do("GET", "/v1/me", session, nil, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("logged-out session still usable = %d", rec.Code)
	}
	if rec := h.do("GET", "/v1/me", otherSession, nil, nil); rec.Code != http.StatusOK {
		t.Fatal("logout revoked a different user's session")
	}
	if rec := h.do("POST", "/v1/auth/logout", session, nil, map[string]string{"Origin": appOrigin}); rec.Code != http.StatusNoContent {
		t.Fatalf("repeated logout = %d", rec.Code)
	}
}
