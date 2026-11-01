package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/middleware"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/requestid"
)

func TestRequestID(t *testing.T) {
	valid := regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)
	for _, inbound := range []string{"", "short", "invalid/identifier", strings.Repeat("a", 65), "request-123._", strings.Repeat("a", 8), strings.Repeat("a", 64)} {
		t.Run(inbound, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("X-Request-Id", inbound)
			rec := httptest.NewRecorder()
			var identity string
			h := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				identity = requestid.From(r.Context())
				if w.Header().Get("X-Request-Id") != identity {
					t.Fatal("identity was not echoed before dispatch")
				}
			}))
			h.ServeHTTP(rec, req)
			if valid.MatchString(inbound) {
				if identity != inbound {
					t.Fatalf("inbound identity changed to %q", identity)
				}
			} else if identity == inbound || !valid.MatchString(identity) {
				t.Fatalf("generated identity = %q", identity)
			}
		})
	}
}
