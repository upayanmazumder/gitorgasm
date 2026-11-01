package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/upayanmazumder/gitorgasm/core/internal/config"
)

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
