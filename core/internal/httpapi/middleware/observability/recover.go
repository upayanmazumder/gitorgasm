package observability

import (
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/response"
)

// Recoverer logs panics and writes internal errors, preserving handler aborts.
func Recoverer(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if err, ok := v.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(v)
				}
				log.ErrorContext(r.Context(), "panic", "value", v, "stack", string(debug.Stack()))
				response.Error(w, r, http.StatusInternalServerError, "internal", "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
