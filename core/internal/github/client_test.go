package github

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/upayanmazumder/gitorgasm/core/internal/config"
)

func TestClassifyRateLimits(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	cases := []struct {
		name   string
		status int
		hdr    map[string]string
		body   string
		kind   ErrorKind
		wait   time.Duration
	}{
		{"retry-after wins", 403, map[string]string{"Retry-After": "30", "X-RateLimit-Remaining": "0"}, `{}`, KindRateLimited, 30 * time.Second},
		{"primary limit waits for reset", 403, map[string]string{"X-RateLimit-Remaining": "0", "X-RateLimit-Reset": strconv.FormatInt(now.Unix()+90, 10)}, `{}`, KindRateLimited, 91 * time.Second},
		{"secondary limit without headers waits a minute", 403, nil, `{"message":"You have exceeded a secondary rate limit"}`, KindRateLimited, time.Minute},
		{"429 is a rate limit", 429, nil, `{}`, KindRateLimited, time.Minute},
		{"plain 403 is a permission error", 403, map[string]string{"X-RateLimit-Remaining": "4999"}, `{"message":"Resource not accessible by integration"}`, KindForbidden, 0},
		{"401", 401, nil, `{}`, KindUnauthorized, 0},
		{"404", 404, nil, `{}`, KindNotFound, 0},
		{"502 is transient", 502, nil, `{}`, KindTransient, 0},
		{"422 is invalid", 422, nil, `{}`, KindInvalid, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp := &http.Response{StatusCode: c.status, Header: http.Header{}}
			for k, v := range c.hdr {
				resp.Header.Set(k, v)
			}
			var err *APIError
			if !errors.As(classify("op", resp, []byte(c.body), now), &err) {
				t.Fatal("classify did not return an APIError")
			}
			if err.Kind != c.kind || err.RetryAfter != c.wait {
				t.Fatalf("kind=%d wait=%s, want kind=%d wait=%s", err.Kind, err.RetryAfter, c.kind, c.wait)
			}
		})
	}
}

func TestNextLink(t *testing.T) {
	h := `<https://api.github.com/x?page=2>; rel="next", <https://api.github.com/x?page=5>; rel="last"`
	if got := nextLink(h); got != "https://api.github.com/x?page=2" {
		t.Fatalf("next = %q", got)
	}
	if got := nextLink(`<https://api.github.com/x?page=1>; rel="prev"`); got != "" {
		t.Fatalf("next on last page = %q", got)
	}
}

func TestLinksOutsideAPIHostAreRefused(t *testing.T) {
	c := New(&config.GitHub{APIBaseURL: "https://api.github.com", RequestTimeout: time.Second})
	if _, err := c.newRequest(context.Background(), "GET", "https://evil.example/installation/repositories", nil); err == nil {
		t.Fatal("followed a link to another host; the token would leak")
	}
	if _, err := c.newRequest(context.Background(), "GET", "https://api.github.com.evil.example/x", nil); err == nil {
		t.Fatal("followed a look-alike host")
	}
	if _, err := c.newRequest(context.Background(), "GET", "https://api.github.com/installation/repositories?page=2", nil); err != nil {
		t.Fatalf("refused a valid link: %v", err)
	}
}

func TestVerifySignature(t *testing.T) {
	// Test vector from GitHub's "Validating webhook deliveries" documentation.
	secret, body := []byte("It's a Secret to Everybody"), []byte("Hello, World!")
	const valid = "sha256=757107ea0eb2509fc211221cce984b8a37570b6d7586c22c46f4379c8b043e17"
	if !VerifySignature(secret, body, valid) {
		t.Fatal("rejected the documented signature")
	}
	for _, h := range []string{"", "sha1=757107ea", "sha256=zz", valid[:len(valid)-2] + "00"} {
		if VerifySignature(secret, body, h) {
			t.Fatalf("accepted header %q", h)
		}
	}
	if VerifySignature([]byte("other"), body, valid) {
		t.Fatal("accepted a signature made with another secret")
	}
}
