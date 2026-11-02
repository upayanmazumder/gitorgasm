package observability_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/middleware/observability"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/requestid"
)

func TestRecoverer(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, nil))
	h := observability.Recoverer(log, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("private panic")
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(requestid.With(req.Context(), "request-123"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var body struct {
		Error struct {
			Code      string `json:"code"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusInternalServerError || body.Error.Code != "internal" ||
		body.Error.RequestID != "request-123" || strings.Contains(rec.Body.String(), "private panic") {
		t.Fatalf("response = %d %s", rec.Code, rec.Body)
	}
	var entry map[string]any
	if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["level"] != "ERROR" || entry["value"] != "private panic" || entry["stack"] == "" {
		t.Fatalf("panic was not logged with its value and stack: %v", entry)
	}
}

func TestRecovererPreservesAbort(t *testing.T) {
	for _, abort := range []error{http.ErrAbortHandler, fmt.Errorf("wrapped: %w", http.ErrAbortHandler)} {
		t.Run(abort.Error(), func(t *testing.T) {
			var logs bytes.Buffer
			log := slog.New(slog.NewJSONHandler(&logs, nil))
			h := observability.Recoverer(log, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				panic(abort)
			}))
			rec := httptest.NewRecorder()
			defer func() {
				if got, ok := recover().(error); !ok || !errors.Is(got, abort) {
					t.Errorf("panic = %v, want original %v", got, abort)
				}
				if logs.Len() != 0 || rec.Body.Len() != 0 {
					t.Errorf("abort produced log %q or response %q", logs.String(), rec.Body.String())
				}
			}()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		})
	}
}
