package middleware

import (
	"net/http"
	"regexp"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/requestid"
	"github.com/upayanmazumder/gitorgasm/core/internal/secure"
)

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)

// RequestID accepts a well-formed inbound X-Request-Id or generates and echoes one.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if !validRequestID.MatchString(id) {
			id = secure.RandomToken()[:22]
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(requestid.With(r.Context(), id)))
	})
}
