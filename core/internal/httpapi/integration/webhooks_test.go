package integration_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func sign(body []byte) string {
	mac := hmac.New(sha256.New, []byte(webhookSecret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestWebhookIngestion(t *testing.T) {
	h := newHarness(t)
	body := []byte(`{"action":"added","installation":{"id":10}}`)
	hdr := func(sig string) map[string]string {
		return map[string]string{
			"X-Hub-Signature-256": sig, "X-GitHub-Delivery": "d-1", "X-GitHub-Event": "installation_repositories",
			"Content-Type": "application/json",
		}
	}
	if rec := h.do("POST", "/webhooks/github", "", body, hdr("sha256=00")); rec.Code != 401 {
		t.Fatalf("bad signature = %d, want 401", rec.Code)
	}
	tampered := []byte(`{"action":"added","installation":{"id":11}}`)
	if rec := h.do("POST", "/webhooks/github", "", tampered, hdr(sign(body))); rec.Code != 401 {
		t.Fatalf("tampered body = %d, want 401", rec.Code)
	}
	first := decode[map[string]any](t, h.do("POST", "/webhooks/github", "", body, hdr(sign(body))))
	second := decode[map[string]any](t, h.do("POST", "/webhooks/github", "", body, hdr(sign(body))))
	if first["duplicate"] != false || second["duplicate"] != true {
		t.Fatalf("first=%v second=%v", first, second)
	}
	var deliveries, queued int
	ctx := context.Background()
	_ = h.pool.QueryRow(ctx, `SELECT count(*) FROM webhook_deliveries WHERE installation_id = 10`).Scan(&deliveries)
	_ = h.pool.QueryRow(ctx, `SELECT count(*) FROM river_job WHERE kind = 'process_webhook'`).Scan(&queued)
	if deliveries != 1 || queued != 1 {
		t.Fatalf("deliveries=%d queued=%d, want 1 and 1", deliveries, queued)
	}
}
