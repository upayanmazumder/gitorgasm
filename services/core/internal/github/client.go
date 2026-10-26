// Package github is a narrow client for the GitHub REST endpoints that Gitorgasm uses.
//
// Each endpoint was verified against github/rest-api-description v2.1.0
// (see docs/architecture/github-integration.md). Add endpoints only after the same check.
package github

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/upayanmazumder/gitorgasm/services/core/internal/config"
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

// Client calls GitHub with App, installation, or user credentials.
type Client struct {
	cfg  *config.GitHub
	http *http.Client
	now  func() time.Time

	mu     sync.Mutex
	tokens map[int64]installationToken
}

type installationToken struct {
	value     string
	expiresAt time.Time
}

// New builds a client. The transport is traced with OpenTelemetry.
func New(cfg *config.GitHub) *Client {
	return &Client{
		cfg:    cfg,
		http:   &http.Client{Timeout: cfg.RequestTimeout, Transport: otelhttp.NewTransport(http.DefaultTransport)},
		now:    time.Now,
		tokens: make(map[int64]installationToken),
	}
}

// ErrorKind classifies GitHub failures for retry decisions.
type ErrorKind int

const (
	KindTransient    ErrorKind = iota // network errors and 5xx: retry with backoff
	KindRateLimited                   // primary or secondary rate limit: wait RetryAfter
	KindUnauthorized                  // 401: credential revoked or expired
	KindForbidden                     // 403 without rate-limit signals: permission missing
	KindNotFound                      // 404: resource gone or not visible to this credential
	KindInvalid                       // other 4xx: request is wrong, do not retry
)

// APIError is a classified GitHub error.
type APIError struct {
	Operation  string
	Status     int
	Kind       ErrorKind
	RetryAfter time.Duration
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("github %s: status %d: %s", e.Operation, e.Status, e.Message)
}

// IsKind reports whether err is an APIError of the given kind.
func IsKind(err error, kind ErrorKind) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Kind == kind
}

// classify maps a response to an error following GitHub's documented rate-limit rules:
// honor retry-after first, then x-ratelimit-reset when remaining is 0, else wait one minute
// for secondary limits.
func classify(op string, resp *http.Response, body []byte, now time.Time) error {
	e := &APIError{Operation: op, Status: resp.StatusCode, Message: summarize(body)}
	remaining := resp.Header.Get("X-RateLimit-Remaining")
	retryAfter := resp.Header.Get("Retry-After")
	switch {
	case resp.StatusCode == http.StatusTooManyRequests ||
		(resp.StatusCode == http.StatusForbidden && (remaining == "0" || retryAfter != "" ||
			strings.Contains(strings.ToLower(e.Message), "rate limit"))):
		e.Kind = KindRateLimited
		switch {
		case retryAfter != "":
			if s, err := strconv.Atoi(retryAfter); err == nil {
				e.RetryAfter = time.Duration(s) * time.Second
			}
		case remaining == "0":
			if reset, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
				e.RetryAfter = time.Unix(reset, 0).Sub(now) + time.Second
			}
		}
		if e.RetryAfter <= 0 {
			e.RetryAfter = time.Minute
		}
	case resp.StatusCode == http.StatusUnauthorized:
		e.Kind = KindUnauthorized
	case resp.StatusCode == http.StatusForbidden:
		e.Kind = KindForbidden
	case resp.StatusCode == http.StatusNotFound:
		e.Kind = KindNotFound
	case resp.StatusCode >= 500:
		e.Kind = KindTransient
	default:
		e.Kind = KindInvalid
	}
	return e
}

func summarize(body []byte) string {
	var m struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &m) == nil && m.Message != "" {
		return m.Message
	}
	if len(body) > 200 {
		body = body[:200]
	}
	return string(body)
}

// do executes a request, records metrics, and decodes a 2xx JSON body into out.
// It returns the response headers so callers can follow Link pagination.
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

// appJWT signs a short-lived JWT that authenticates as the GitHub App.
// iat is backdated 60 s to tolerate clock drift. GitHub rejects exp more than 10 min ahead.
func (c *Client) appJWT() (string, error) {
	now := c.now()
	claims := jwt.RegisteredClaims{
		Issuer:    strconv.FormatInt(c.cfg.AppID, 10),
		IssuedAt:  jwt.NewNumericDate(now.Add(-60 * time.Second)),
		ExpiresAt: jwt.NewNumericDate(now.Add(9 * time.Minute)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(c.cfg.PrivateKey)
}

// newRequest resolves path against the API base URL. Absolute URLs come only from Link
// headers and must stay on the API host, so credentials never leave GitHub.
func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		if !strings.HasPrefix(path, c.cfg.APIBaseURL+"/") {
			return nil, fmt.Errorf("refusing to follow link outside %s", c.cfg.APIBaseURL)
		}
		return http.NewRequestWithContext(ctx, method, path, body)
	}
	return http.NewRequestWithContext(ctx, method, c.cfg.APIBaseURL+path, body)
}

// GetInstallation calls GET /app/installations/{installation_id} (apps/get-installation).
func (c *Client) GetInstallation(ctx context.Context, id int64) (*Installation, error) {
	token, err := c.appJWT()
	if err != nil {
		return nil, err
	}
	req, err := c.newRequest(ctx, http.MethodGet, "/app/installations/"+strconv.FormatInt(id, 10), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	var inst Installation
	if _, err := c.do(ctx, "apps/get-installation", "app", req, &inst); err != nil {
		return nil, err
	}
	return &inst, nil
}

// installationToken returns a cached token or mints one with
// POST /app/installations/{installation_id}/access_tokens (apps/create-installation-access-token).
// Tokens live one hour. The cache refreshes them five minutes before expiry.
func (c *Client) installationToken(ctx context.Context, id int64) (string, error) {
	c.mu.Lock()
	t, ok := c.tokens[id]
	c.mu.Unlock()
	if ok && c.now().Before(t.expiresAt.Add(-5*time.Minute)) {
		return t.value, nil
	}
	jwtToken, err := c.appJWT()
	if err != nil {
		return "", err
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/app/installations/"+strconv.FormatInt(id, 10)+"/access_tokens", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	var out struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if _, err := c.do(ctx, "apps/create-installation-access-token", "app", req, &out); err != nil {
		return "", err
	}
	c.mu.Lock()
	c.tokens[id] = installationToken{value: out.Token, expiresAt: out.ExpiresAt}
	c.mu.Unlock()
	return out.Token, nil
}

// ForgetInstallation drops a cached token, for example after the installation is deleted.
func (c *Client) ForgetInstallation(id int64) {
	c.mu.Lock()
	delete(c.tokens, id)
	c.mu.Unlock()
}

// ListInstallationRepositories pages through GET /installation/repositories
// (apps/list-repos-accessible-to-installation) and calls fn once per page.
func (c *Client) ListInstallationRepositories(ctx context.Context, id int64, fn func([]Repository) error) error {
	next := "/installation/repositories?per_page=100"
	for next != "" {
		token, err := c.installationToken(ctx, id)
		if err != nil {
			return err
		}
		req, err := c.newRequest(ctx, http.MethodGet, next, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		var page struct {
			Repositories []Repository `json:"repositories"`
		}
		hdr, err := c.do(ctx, "apps/list-repos-accessible-to-installation", "installation", req, &page)
		if err != nil {
			return err
		}
		if err := fn(page.Repositories); err != nil {
			return err
		}
		next = nextLink(hdr.Get("Link"))
	}
	return nil
}

// GetAuthenticatedUser calls GET /user (users/get-authenticated) with a user access token.
func (c *Client) GetAuthenticatedUser(ctx context.Context, userToken string) (*User, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+userToken)
	var u User
	if _, err := c.do(ctx, "users/get-authenticated", "user", req, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// ListUserInstallations pages through GET /user/installations
// (apps/list-installations-for-authenticated-user). GitHub returns only installations of this
// App that the user can access. This is the tenant authorization source.
func (c *Client) ListUserInstallations(ctx context.Context, userToken string) ([]Installation, error) {
	var all []Installation
	next := "/user/installations?per_page=100"
	for next != "" {
		req, err := c.newRequest(ctx, http.MethodGet, next, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+userToken)
		var page struct {
			Installations []Installation `json:"installations"`
		}
		hdr, err := c.do(ctx, "apps/list-installations-for-authenticated-user", "user", req, &page)
		if err != nil {
			return nil, err
		}
		all = append(all, page.Installations...)
		next = nextLink(hdr.Get("Link"))
	}
	return all, nil
}

// AuthorizeURL builds the web application flow URL with state and a PKCE S256 challenge.
func (c *Client) AuthorizeURL(state, codeChallenge string) string {
	q := url.Values{}
	q.Set("client_id", c.cfg.ClientID)
	q.Set("redirect_uri", c.cfg.OAuthCallback)
	q.Set("state", state)
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", "S256")
	q.Set("allow_signup", "false")
	return c.cfg.WebBaseURL + "/login/oauth/authorize?" + q.Encode()
}

// InstallURL is where users install the App on an organization.
func (c *Client) InstallURL() string { return c.cfg.InstallURL }

// ExchangeCode exchanges an OAuth code for a user access token at
// POST https://github.com/login/oauth/access_token.
func (c *Client) ExchangeCode(ctx context.Context, code, codeVerifier string) (*UserToken, error) {
	form := url.Values{}
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", c.cfg.ClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", c.cfg.OAuthCallback)
	form.Set("code_verifier", codeVerifier)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.WebBaseURL+"/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var out struct {
		UserToken
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if _, err := c.do(ctx, "oauth/access-token", "oauth", req, &out); err != nil {
		return nil, err
	}
	// GitHub reports OAuth failures with HTTP 200 and an error field.
	if out.Error != "" || out.AccessToken == "" {
		return nil, &APIError{Operation: "oauth/access-token", Status: http.StatusOK, Kind: KindInvalid, Message: out.Error}
	}
	if out.ExpiresIn > 0 {
		out.ExpiresAt = c.now().Add(time.Duration(out.ExpiresIn) * time.Second)
	}
	return &out.UserToken, nil
}

// nextLink extracts the rel="next" URL from an RFC 8288 Link header.
func nextLink(header string) string {
	for part := range strings.SplitSeq(header, ",") {
		segs := strings.Split(part, ";")
		if len(segs) < 2 {
			continue
		}
		for _, s := range segs[1:] {
			if strings.TrimSpace(s) == `rel="next"` {
				return strings.Trim(strings.TrimSpace(segs[0]), "<>")
			}
		}
	}
	return ""
}

// VerifySignature checks X-Hub-Signature-256 (HMAC-SHA256 of the raw body) in constant time.
func VerifySignature(secret, body []byte, header string) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	got, err := hex.DecodeString(header[len(prefix):])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return hmac.Equal(got, mac.Sum(nil))
}
