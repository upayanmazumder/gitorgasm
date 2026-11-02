package observability

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/felixge/httpsnoop"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/requestid"
)

// AccessLog records HTTP metrics and logs requests except health probes.
func AccessLog(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metrics := httpsnoop.CaptureMetrics(next, w, r)
		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		httpRequests.WithLabelValues(route, r.Method, strconv.Itoa(metrics.Code)).Inc()
		httpDuration.WithLabelValues(route).Observe(metrics.Duration.Seconds())
		if route == "GET /healthz" || route == "GET /readyz" {
			return
		}
		log.InfoContext(r.Context(), "request", "method", r.Method, "route", route, "status", metrics.Code,
			"duration_ms", metrics.Duration.Milliseconds(), "request_id", requestid.From(r.Context()))
	})
}
