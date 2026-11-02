package integration_test

import (
	"testing"
)

func TestScoreQueriesRejectInvalidInput(t *testing.T) {
	h, alice, _, _, _ := newScoreFixture(t)
	for _, bad := range []string{`{"period":{"kind":"custom","start":"2026-01-02T00:00:00Z","end":"2026-01-01T00:00:00Z"}}`, `{"filter":{"preset":"nope"}}`, `{"extra":1}`, `[]`} {
		if rec := h.do("POST", "/v1/organizations/acme/scores/query", alice, []byte(bad), map[string]string{"Origin": appOrigin}); rec.Code != 400 {
			t.Fatalf("invalid query %s = %d %s", bad, rec.Code, rec.Body)
		}
	}
}
