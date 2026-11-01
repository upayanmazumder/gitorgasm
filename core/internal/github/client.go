// Package github wraps REST endpoints verified against github/rest-api-description v2.1.0.
package github

import (
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/upayanmazumder/gitorgasm/core/internal/config"
)

// Client calls GitHub with App, installation, or user credentials.
type Client struct {
	cfg  *config.GitHub
	http *http.Client
	now  func() time.Time

	mu     sync.Mutex
	tokens map[int64]installationToken
}

// New builds an OpenTelemetry-instrumented GitHub client.
func New(cfg *config.GitHub) *Client {
	return &Client{
		cfg:    cfg,
		http:   &http.Client{Timeout: cfg.RequestTimeout, Transport: otelhttp.NewTransport(http.DefaultTransport)},
		now:    time.Now,
		tokens: make(map[int64]installationToken),
	}
}
