package httpapi_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/upayanmazumder/gitorgasm/core/internal/config"
	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs"
	"github.com/upayanmazumder/gitorgasm/core/internal/secure"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
	"github.com/upayanmazumder/gitorgasm/core/internal/testdb"
)

const (
	appOrigin     = "https://app.example.test"
	webhookSecret = "test-webhook-secret"
)

type harness struct {
	t      *testing.T
	pool   *pgxpool.Pool
	srv    http.Handler
	router routers.Router
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	pool := testdb.New(t)
	jobClient, err := jobs.NewInsertClient(pool)
	if err != nil {
		t.Fatal(err)
	}
	sealer, err := secure.NewSealer(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	origin, _ := url.Parse(appOrigin)
	gh := &config.GitHub{WebhookSecret: []byte(webhookSecret), APIBaseURL: "http://127.0.0.1:1", WebBaseURL: "http://127.0.0.1:1"}
	cfg := &config.Config{AppOrigin: origin, CookieSecure: true, SessionTTL: time.Hour, GitHub: gh}
	s := &httpapi.Server{
		Pool: pool, Jobs: jobClient, Sealer: sealer, Cfg: cfg, GitHub: github.New(gh),
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Now: time.Now,
	}

	doc, err := openapi3.NewLoader().LoadFromFile("../../../contracts/openapi/v1.yaml")
	if err != nil {
		t.Fatalf("load contract: %v", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("contract is invalid: %v", err)
	}
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		t.Fatal(err)
	}
	return &harness{t: t, pool: pool, srv: s.Handler(), router: router}
}

// do sends a request and validates the response against the OpenAPI contract.
func (h *harness) do(method, path, session string, body []byte, hdr map[string]string) *httptest.ResponseRecorder {
	h.t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if session != "" {
		req.AddCookie(&http.Cookie{Name: "__Host-gitorgasm_session", Value: session})
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.srv.ServeHTTP(rec, req)

	route, params, err := h.router.FindRoute(req)
	if err != nil {
		h.t.Fatalf("%s %s is not in the contract: %v", method, path, err)
	}
	in := &openapi3filter.RequestValidationInput{
		Request: req, PathParams: params, Route: route,
		Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
	}
	out := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: in, Status: rec.Code, Header: rec.Header(),
		Body:    io.NopCloser(bytes.NewReader(rec.Body.Bytes())),
		Options: &openapi3filter.Options{IncludeResponseStatus: true},
	}
	if err := openapi3filter.ValidateResponse(context.Background(), out); err != nil {
		h.t.Fatalf("%s %s response violates the contract: %v\nbody: %s", method, path, err, rec.Body.String())
	}
	return rec
}

func (h *harness) exec(sql string, args ...any) {
	h.t.Helper()
	if _, err := h.pool.Exec(context.Background(), sql, args...); err != nil {
		h.t.Fatalf("exec %q: %v", sql, err)
	}
}

// seedTenant grants a user access to an organization installation with repositories.
func (h *harness) seedTenant(userGitHubID int64, org string, orgID, instID int64, repoNames ...string) string {
	h.t.Helper()
	ctx := context.Background()
	userID, err := store.UpsertUser(ctx, h.pool, &github.User{ID: userGitHubID, Login: "user" + org})
	if err != nil {
		h.t.Fatal(err)
	}
	inst := &github.Installation{ID: instID, RepositorySelection: "all", Account: &github.Account{ID: orgID, Login: org, Type: "Organization"}}
	if err := store.UpsertInstallation(ctx, h.pool, inst); err != nil {
		h.t.Fatal(err)
	}
	if err := store.ReplaceUserInstallations(ctx, h.pool, userID, []int64{instID}); err != nil {
		h.t.Fatal(err)
	}
	repos := make([]github.Repository, len(repoNames))
	for i, n := range repoNames {
		repos[i] = github.Repository{ID: instID*1000 + int64(i), Name: n, FullName: org + "/" + n, Owner: github.Owner{ID: orgID, Login: org}, Visibility: "private", Private: true}
	}
	if err := store.UpsertRepositories(ctx, h.pool, instID, repos, time.Now()); err != nil {
		h.t.Fatal(err)
	}
	session := secure.RandomToken()
	if err := store.CreateSession(ctx, h.pool, store.NewSession{
		IDHash: secure.Hash(session), UserID: userID, KeyVersion: 1, TokenSealed: []byte{1}, ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		h.t.Fatal(err)
	}
	return session
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}
	return v
}

type page struct {
	Data []struct {
		Login           string `json:"login"`
		Name            string `json:"name"`
		RepositoryCount int    `json:"repository_count"`
	} `json:"data"`
	NextCursor *string `json:"next_cursor"`
}

func TestHealthAndReadiness(t *testing.T) {
	h := newHarness(t)
	if rec := h.do("GET", "/healthz", "", nil, nil); rec.Code != 200 {
		t.Fatalf("healthz = %d", rec.Code)
	}
	if rec := h.do("GET", "/readyz", "", nil, nil); rec.Code != 200 {
		t.Fatalf("readyz = %d", rec.Code)
	}
	h.pool.Close()
	if rec := h.do("GET", "/readyz", "", nil, nil); rec.Code != 503 {
		t.Fatalf("readyz with closed pool = %d, want 503", rec.Code)
	}
}

func TestOrganizationsAreTenantScoped(t *testing.T) {
	h := newHarness(t)
	alice := h.seedTenant(1, "acme", 100, 10, "api", "web", "infra")
	bob := h.seedTenant(2, "globex", 200, 20, "secret")

	if rec := h.do("GET", "/v1/organizations", "", nil, nil); rec.Code != 401 {
		t.Fatalf("anonymous = %d, want 401", rec.Code)
	}

	rec := h.do("GET", "/v1/organizations", alice, nil, nil)
	got := decode[page](t, rec)
	if rec.Code != 200 || len(got.Data) != 1 || got.Data[0].Login != "acme" || got.Data[0].RepositoryCount != 3 {
		t.Fatalf("alice orgs = %d %s", rec.Code, rec.Body)
	}

	// Alice must not see Globex, and the response must not reveal that it exists.
	rec = h.do("GET", "/v1/organizations/globex/repositories", alice, nil, nil)
	if rec.Code != 404 {
		t.Fatalf("cross-tenant read = %d, want 404", rec.Code)
	}
	rec = h.do("GET", "/v1/organizations/globex/repositories", bob, nil, nil)
	if rec.Code != 200 || len(decode[page](t, rec).Data) != 1 {
		t.Fatalf("bob own org = %d %s", rec.Code, rec.Body)
	}

	// Revoking the grant (as a re-login with fewer installations does) removes access at once.
	h.exec(`DELETE FROM user_installations WHERE installation_id = 10`)
	if rec := h.do("GET", "/v1/organizations/acme/repositories", alice, nil, nil); rec.Code != 404 {
		t.Fatalf("after revocation = %d, want 404", rec.Code)
	}
	// A deleted installation hides the organization even if a grant row remains.
	h.exec(`UPDATE installations SET deleted_at = now() WHERE github_id = 20`)
	if rec := h.do("GET", "/v1/organizations", bob, nil, nil); len(decode[page](t, rec).Data) != 0 {
		t.Fatalf("deleted installation still listed: %s", rec.Body)
	}
}

func TestReusedLoginResolvesToNewestAccount(t *testing.T) {
	h := newHarness(t)
	// Account 100 was renamed on GitHub, and account 300 took its login before 100 resynced.
	h.seedTenant(1, "acme", 100, 10, "stale")
	h.exec(`UPDATE accounts SET updated_at = now() - interval '1 day' WHERE github_id = 100`)
	session := h.seedTenant(1, "ACME", 300, 30, "fresh")
	h.exec(`INSERT INTO user_installations (user_id, installation_id) SELECT id, 10 FROM users WHERE github_id = 1`)

	rec := h.do("GET", "/v1/organizations/acme/repositories", session, nil, nil)
	got := decode[page](t, rec)
	if rec.Code != 200 || len(got.Data) != 1 || got.Data[0].Name != "fresh" {
		t.Fatalf("acme repositories = %d %s, want the newest account's repositories", rec.Code, rec.Body)
	}
}

func TestRepositoryPagination(t *testing.T) {
	h := newHarness(t)
	s := h.seedTenant(1, "acme", 100, 10, "Charlie", "alpha", "bravo", "delta", "echo")
	h.exec(`UPDATE repositories SET deleted_at = now() WHERE name = 'delta'`)

	var names []string
	cursor := ""
	for range 5 {
		path := "/v1/organizations/ACME/repositories?limit=2"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		rec := h.do("GET", path, s, nil, nil)
		if rec.Code != 200 {
			t.Fatalf("page = %d %s", rec.Code, rec.Body)
		}
		p := decode[page](t, rec)
		for _, r := range p.Data {
			names = append(names, r.Name)
		}
		if p.NextCursor == nil {
			break
		}
		cursor = *p.NextCursor
	}
	want := []string{"alpha", "bravo", "Charlie", "echo"}
	if len(names) != len(want) {
		t.Fatalf("names = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names = %v, want %v", names, want)
		}
	}

	for _, bad := range []string{"limit=0", "limit=101", "limit=x", "cursor=!!!", "cursor=e30"} {
		if rec := h.do("GET", "/v1/organizations/acme/repositories?"+bad, s, nil, nil); rec.Code != 400 {
			t.Errorf("%s = %d, want 400", bad, rec.Code)
		}
	}
}

func TestExpiredSessionIsRejected(t *testing.T) {
	h := newHarness(t)
	s := h.seedTenant(1, "acme", 100, 10)
	h.exec(`UPDATE sessions SET expires_at = now() - interval '1 second'`)
	if rec := h.do("GET", "/v1/me", s, nil, nil); rec.Code != 401 {
		t.Fatalf("expired session = %d, want 401", rec.Code)
	}
}

func TestSyncRequestChecksOriginAndDeduplicates(t *testing.T) {
	h := newHarness(t)
	s := h.seedTenant(1, "acme", 100, 10)

	if rec := h.do("POST", "/v1/organizations/acme/sync", s, nil, map[string]string{"Origin": "https://evil.example"}); rec.Code != 403 {
		t.Fatalf("cross-origin = %d, want 403", rec.Code)
	}
	origin := map[string]string{"Origin": appOrigin}
	first := decode[map[string]any](t, h.do("POST", "/v1/organizations/acme/sync", s, nil, origin))
	second := decode[map[string]any](t, h.do("POST", "/v1/organizations/acme/sync", s, nil, origin))
	if first["duplicate"] != false || second["duplicate"] != true || first["job_id"] != second["job_id"] {
		t.Fatalf("first=%v second=%v", first, second)
	}
	var audits int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_events WHERE action = 'installation.sync_requested'`).Scan(&audits); err != nil || audits != 2 {
		t.Fatalf("audit rows = %d (%v), want 2", audits, err)
	}
	if rec := h.do("POST", "/v1/organizations/globex/sync", s, nil, origin); rec.Code != 404 {
		t.Fatalf("sync of foreign org = %d, want 404", rec.Code)
	}
}

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
