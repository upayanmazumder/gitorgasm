package webhook

import (
	"bufio"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/upayanmazumder/gitorgasm/core/internal/config"
)

// stall opens a connection that announces a body and never sends it.
func stall(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_, err = io.WriteString(conn, "POST /webhooks/github HTTP/1.1\r\nHost: x\r\nContent-Length: 1000\r\n\r\nx")
	if err != nil {
		t.Fatal(err)
	}
	return conn
}

func post(t *testing.T, url string) int {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

// The server has no ReadTimeout, so only the handler's own read deadline can free a stalled slot.
func TestStalledBodiesFreeSlotsAtDeadlineAndFullLimiterRejectsAtOnce(t *testing.T) {
	h := &Handler{
		Cfg:         &config.Config{GitHub: &config.GitHub{WebhookSecret: []byte("secret")}},
		Log:         slog.New(slog.DiscardHandler),
		bodyTimeout: 300 * time.Millisecond,
	}
	srv := httptest.NewServer(otelhttp.NewHandler(h.Handler(), "http"))
	t.Cleanup(srv.Close)
	addr := strings.TrimPrefix(srv.URL, "http://")

	start := time.Now()
	conns := make([]net.Conn, maxWebhookReads)
	for i := range conns {
		conns[i] = stall(t, addr)
	}
	// Both slots are taken once the stalled requests reach the handler; poll for the 503.
	var rejectedIn time.Duration
	for deadline := time.Now().Add(time.Second); ; {
		began := time.Now()
		if post(t, srv.URL) == http.StatusServiceUnavailable {
			rejectedIn = time.Since(began)
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("full limiter never returned 503")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if rejectedIn > 200*time.Millisecond {
		t.Fatalf("503 took %s; it must not queue", rejectedIn)
	}

	for _, conn := range conns {
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
		if err != nil {
			t.Fatalf("stalled request got no response: %v", err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("stalled request status = %d, want 400", resp.StatusCode)
		}
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("slots were held for %s despite a %s body deadline", elapsed, h.bodyTimeout)
	}

	// The slots are free again: a complete unsigned request now reaches signature verification.
	var status int
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if status = post(t, srv.URL); status != http.StatusServiceUnavailable {
			break
		}
	}
	if status != http.StatusUnauthorized {
		t.Fatalf("request after the deadline = %d, want 401", status)
	}
}
