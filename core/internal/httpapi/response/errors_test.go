package response_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/requestid"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/response"
)

func TestInternalError(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, nil))
	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.Pattern = "GET /private"
	req = req.WithContext(requestid.With(req.Context(), "request-123"))
	rec := httptest.NewRecorder()
	response.InternalError(log, rec, req, errors.New("private database failure"))
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
		body.Error.RequestID != "request-123" || strings.Contains(rec.Body.String(), "private database failure") {
		t.Fatalf("response = %d %s", rec.Code, rec.Body)
	}
	var entry map[string]any
	if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["error"] != "private database failure" || entry["route"] != req.Pattern || entry["request_id"] != "request-123" {
		t.Fatalf("log = %v", entry)
	}
}
