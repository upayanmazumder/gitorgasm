package httpapi

import (
	"encoding/json"
	"net/http"
)

// apiError is the single error body shape for every endpoint.
type apiError struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, msg string) {
	var e apiError
	e.Error.Code, e.Error.Message, e.Error.RequestID = code, msg, RequestIDFrom(r.Context())
	writeJSON(w, status, e)
}

func (s *Server) internalError(w http.ResponseWriter, r *http.Request, err error) {
	s.Log.ErrorContext(r.Context(), "request failed", "error", err, "route", r.Pattern, "request_id", RequestIDFrom(r.Context()))
	writeError(w, r, http.StatusInternalServerError, "internal", "internal error")
}
