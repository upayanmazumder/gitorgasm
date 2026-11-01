package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/middleware"
)

func TestCheckOrigin(t *testing.T) {
	const origin = "https://app.example:8443"
	tests := []struct {
		name, method, path, origin string
		allowed                    bool
	}{
		{"get", http.MethodGet, "/v1/me", "https://other.example", true},
		{"head", http.MethodHead, "/v1/me", "", true},
		{"options", http.MethodOptions, "/v1/me", "", true},
		{"same origin", http.MethodPost, "/v1/auth/logout", origin, true},
		{"missing origin", http.MethodPost, "/v1/auth/logout", "", false},
		{"foreign origin", http.MethodPost, "/v1/auth/logout", "https://other.example", false},
		{"different port", http.MethodPut, "/v1/preset", "https://app.example", false},
		{"trailing slash", http.MethodDelete, "/v1/preset", origin + "/", false},
		{"webhook", http.MethodPost, "/webhooks/github", "", true},
		{"webhook trailing slash", http.MethodPost, "/webhooks/github/", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			h := middleware.CheckOrigin(origin, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			}))
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Header.Set("Origin", tt.origin)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if called != tt.allowed {
				t.Fatalf("called = %t, want %t", called, tt.allowed)
			}
			if !tt.allowed {
				var body struct {
					Error struct {
						Code string `json:"code"`
					} `json:"error"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if rec.Code != http.StatusForbidden || body.Error.Code != "origin_rejected" {
					t.Fatalf("rejection = %d %s", rec.Code, rec.Body)
				}
			}
		})
	}
}
