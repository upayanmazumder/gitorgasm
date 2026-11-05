package observability_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/middleware/observability"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/requestid"
)

// requestCount reads the request counter and fails when its labels change.
func requestCount(t *testing.T, route, method, status string) float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() != "gitorgasm_http_requests_total" {
			continue
		}
		for _, metric := range family.GetMetric() {
			labels := map[string]string{}
			var names []string
			for _, label := range metric.GetLabel() {
				labels[label.GetName()] = label.GetValue()
				names = append(names, label.GetName())
			}
			slices.Sort(names)
			if !slices.Equal(names, []string{"method", "route", "status"}) {
				t.Fatalf("request counter labels = %v", names)
			}
			if labels["route"] == route && labels["method"] == method && labels["status"] == status {
				return metric.GetCounter().GetValue()
			}
		}
	}
	return 0
}

func TestAccessLog(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		handler http.HandlerFunc
		route   string
		status  int
		logged  bool
	}{
		{"committed status survives flush", "GET /t/flush", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			w.(http.Flusher).Flush()
			w.WriteHeader(http.StatusInternalServerError)
		}, "GET /t/flush", http.StatusAccepted, true},
		{"default status", "GET /t/default", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("ok"))
		}, "GET /t/default", http.StatusOK, true},
		{"unmatched route", "", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}, "unmatched", http.StatusNotFound, true},
		{"liveness probe is not logged", "GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}, "GET /healthz", http.StatusOK, false},
		{"readiness probe is not logged", "GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}, "GET /readyz", http.StatusServiceUnavailable, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer
			log := slog.New(slog.NewJSONHandler(&logs, nil))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Pattern = tt.pattern
			req = req.WithContext(requestid.With(req.Context(), "request-123"))
			statusLabel := strconv.Itoa(tt.status)
			start := requestCount(t, tt.route, http.MethodGet, statusLabel)

			observability.AccessLog(log, tt.handler).ServeHTTP(httptest.NewRecorder(), req)

			if got := requestCount(t, tt.route, http.MethodGet, statusLabel); got != start+1 {
				t.Fatalf("requests{route=%q,status=%s} = %v, want %v", tt.route, statusLabel, got, start+1)
			}
			if !tt.logged {
				if logs.Len() != 0 {
					t.Fatalf("probe was logged: %s", &logs)
				}
				return
			}
			var entry map[string]any
			if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
				t.Fatal(err)
			}
			if entry["route"] != tt.route || entry["status"] != float64(tt.status) || entry["request_id"] != "request-123" {
				t.Fatalf("log = %v", entry)
			}
		})
	}
}

func TestAccessLogBoundsMethodLabel(t *testing.T) {
	tests := []struct{ method, label string }{
		{http.MethodDelete, http.MethodDelete},
		{"BREW", "other"},
		{"get", "other"},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			const route = "GET /t/method"
			start := requestCount(t, route, tt.label, "200")
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Method = tt.method
			req.Pattern = route

			observability.AccessLog(slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(httptest.NewRecorder(), req)

			if got := requestCount(t, route, tt.label, "200"); got != start+1 {
				t.Fatalf("requests{method=%q} = %v, want %v", tt.label, got, start+1)
			}
		})
	}
}
