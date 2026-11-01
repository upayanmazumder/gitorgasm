package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// APIVersion is the REST API version header value that matches the pinned description.
const APIVersion = "2022-11-28"

const maxResponseBytes = 32 << 20

var (
	requestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gitorgasm_github_requests_total",
		Help: "GitHub API requests by operation and HTTP status class.",
	}, []string{"operation", "status"})
	rateRemaining = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gitorgasm_github_rate_limit_remaining",
		Help: "Last observed x-ratelimit-remaining by credential kind.",
	}, []string{"credential"})
)

func (c *Client) do(ctx context.Context, op, credential string, req *http.Request, out any) (http.Header, error) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", APIVersion)
	req.Header.Set("User-Agent", "gitorgasm")
	resp, err := c.http.Do(req)
	if err != nil {
		requestsTotal.WithLabelValues(op, "error").Inc()
		return nil, &APIError{Operation: op, Kind: KindTransient, Message: err.Error()}
	}
	defer func() { _ = resp.Body.Close() }()
	requestsTotal.WithLabelValues(op, strconv.Itoa(resp.StatusCode/100)+"xx").Inc()
	if v, err := strconv.ParseFloat(resp.Header.Get("X-RateLimit-Remaining"), 64); err == nil {
		rateRemaining.WithLabelValues(credential).Set(v)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, &APIError{Operation: op, Kind: KindTransient, Message: err.Error()}
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, classify(op, resp, body, c.now())
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return nil, &APIError{Operation: op, Status: resp.StatusCode, Kind: KindInvalid, Message: "decode: " + err.Error()}
		}
	}
	return resp.Header, nil
}

// newRequest confines absolute pagination URLs to the API host so credentials cannot leak.
func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		if !strings.HasPrefix(path, c.cfg.APIBaseURL+"/") {
			return nil, fmt.Errorf("refusing to follow link outside %s", c.cfg.APIBaseURL)
		}
		return http.NewRequestWithContext(ctx, method, path, body)
	}
	return http.NewRequestWithContext(ctx, method, c.cfg.APIBaseURL+path, body)
}
