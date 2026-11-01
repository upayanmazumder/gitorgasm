package httpapi

import (
	"net/http"
	"strconv"

	"github.com/felixge/httpsnoop"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gitorgasm_http_requests_total",
		Help: "HTTP requests by route pattern, method, and status code.",
	}, []string{"route", "method", "status"})
	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "gitorgasm_http_request_duration_seconds",
		Help:    "HTTP request latency by route pattern.",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
	}, []string{"route"})
)

func (s *Server) accessLog(next http.Handler) http.Handler {
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
		s.Log.InfoContext(r.Context(), "request", "method", r.Method, "route", route, "status", metrics.Code,
			"duration_ms", metrics.Duration.Milliseconds(), "request_id", RequestIDFrom(r.Context()))
	})
}
