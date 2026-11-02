package integration_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
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

func assertAuthRows(t *testing.T, h *harness, sessions, grants, jobs int) {
	t.Helper()
	var gotSessions, gotGrants, gotJobs int
	err := h.pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM sessions), (SELECT count(*) FROM user_installations), (SELECT count(*) FROM river_job WHERE kind = 'sync_installation')`).Scan(&gotSessions, &gotGrants, &gotJobs)
	if err != nil || gotSessions != sessions || gotGrants != grants || gotJobs != jobs {
		t.Fatalf("sessions/grants/jobs = %d/%d/%d (%v), want %d/%d/%d", gotSessions, gotGrants, gotJobs, err, sessions, grants, jobs)
	}
}
