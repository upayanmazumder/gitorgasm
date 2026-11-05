package admission

import (
	"net/http"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/response"
)

// LimitConcurrent bounds concurrent calls. A call that finds no free slot is rejected at once
// with 503 instead of queueing, so a few stalled callers cannot build up a backlog.
func LimitConcurrent(n int, next http.HandlerFunc) http.HandlerFunc {
	slots := make(chan struct{}, n)
	return func(w http.ResponseWriter, r *http.Request) {
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
			next(w, r)
		default:
			response.Error(w, r, http.StatusServiceUnavailable, "busy", "too many concurrent requests; try again")
		}
	}
}
