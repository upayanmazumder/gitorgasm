package response

import (
	"log/slog"
	"net/http"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/requestid"
)

type apiError struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	} `json:"error"`
}

// Error writes the shared error envelope with the current request identity.
func Error(w http.ResponseWriter, r *http.Request, status int, code, msg string) {
	var e apiError
	e.Error.Code, e.Error.Message, e.Error.RequestID = code, msg, requestid.From(r.Context())
	JSON(w, status, e)
}

// InternalError logs a request failure and writes the generic internal error.
func InternalError(log *slog.Logger, w http.ResponseWriter, r *http.Request, err error) {
	log.ErrorContext(r.Context(), "request failed", "error", err, "route", r.Pattern, "request_id", requestid.From(r.Context()))
	Error(w, r, http.StatusInternalServerError, "internal", "internal error")
}
