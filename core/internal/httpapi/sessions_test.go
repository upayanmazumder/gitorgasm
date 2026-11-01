package httpapi_test

import (
	"net/http"
	"testing"
)

func TestExpiredSessionIsRejected(t *testing.T) {
	h := newHarness(t)
	s := h.seedTenant(1, "acme", 100, 10)
	h.exec(`UPDATE sessions SET expires_at = now() - interval '1 second'`)
	if rec := h.do("GET", "/v1/me", s, nil, nil); rec.Code != 401 {
		t.Fatalf("expired session = %d, want 401", rec.Code)
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
