package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/response"
)

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// readyz requires PostgreSQL before Kubernetes routes traffic to the pod.
func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.Pool.Ping(ctx); err != nil {
		response.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "database": "unreachable"})
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "ready", "database": "ok"})
}
