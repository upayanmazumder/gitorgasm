package middleware

import (
	"net/http"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/response"
)

// CheckOrigin guards state-changing browser requests; webhooks use HMAC instead.
func CheckOrigin(allowed string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		unsafe := r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions
		if unsafe && r.URL.Path != "/webhooks/github" && r.Header.Get("Origin") != allowed {
			response.Error(w, r, http.StatusForbidden, "origin_rejected", "cross-origin request rejected")
			return
		}
		next.ServeHTTP(w, r)
	})
}
