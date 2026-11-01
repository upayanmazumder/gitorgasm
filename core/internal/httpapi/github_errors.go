package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/upayanmazumder/gitorgasm/core/internal/github"
)

// githubError maps upstream failures to stable API errors without leaking upstream bodies.
func (s *Server) githubError(w http.ResponseWriter, r *http.Request, err error) {
	var apiErr *github.APIError
	if errors.As(err, &apiErr) {
		s.Log.WarnContext(r.Context(), "github call failed", "operation", apiErr.Operation, "status", apiErr.Status, "kind", int(apiErr.Kind))
		switch apiErr.Kind {
		case github.KindRateLimited:
			w.Header().Set("Retry-After", strconv.Itoa(int(apiErr.RetryAfter.Seconds())))
			writeError(w, r, http.StatusServiceUnavailable, "github_rate_limited", "GitHub rate limit reached; retry later")
			return
		case github.KindInvalid, github.KindUnauthorized:
			writeError(w, r, http.StatusBadRequest, "github_auth_failed", "GitHub rejected the authorization; start again")
			return
		}
	}
	s.Log.ErrorContext(r.Context(), "github call failed", "error", err)
	writeError(w, r, http.StatusBadGateway, "github_unavailable", "GitHub request failed")
}
