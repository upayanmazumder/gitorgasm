package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/upayanmazumder/gitorgasm/core/internal/config"
	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs"
	"github.com/upayanmazumder/gitorgasm/core/internal/scorer"
	"github.com/upayanmazumder/gitorgasm/core/internal/secure"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/identity"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/tenancy"
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
	return newHarnessWithGitHub(t, &config.GitHub{
		WebhookSecret: []byte(webhookSecret), APIBaseURL: "http://127.0.0.1:1", WebBaseURL: "http://127.0.0.1:1",
	})
}

func newHarnessWithGitHub(t *testing.T, gh *config.GitHub) *harness {
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
	cfg := &config.Config{AppOrigin: origin, CookieSecure: true, SessionTTL: time.Hour, GitHub: gh}
	s := &httpapi.Server{
		Pool: pool, Jobs: jobClient, Sealer: sealer, Cfg: cfg, GitHub: github.New(gh),
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Now: time.Now, Scorer: testScorer(),
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

// testScorer runs the binary that `make test` builds and exports as SCORING_BIN.
func testScorer() *scorer.Runner {
	return scorer.New(os.Getenv("SCORING_BIN"), 2, 5*time.Second)
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
	userID, err := identity.UpsertUser(ctx, h.pool, &models.User{ID: userGitHubID, Login: "user" + org})
	if err != nil {
		h.t.Fatal(err)
	}
	inst := &models.Installation{ID: instID, RepositorySelection: "all", Account: &models.Account{ID: orgID, Login: org, Type: "Organization"}}
	if err := tenancy.UpsertInstallation(ctx, h.pool, inst); err != nil {
		h.t.Fatal(err)
	}
	if err := tenancy.ReplaceUserInstallations(ctx, h.pool, userID, []int64{instID}); err != nil {
		h.t.Fatal(err)
	}
	repos := make([]models.Repository, len(repoNames))
	for i, n := range repoNames {
		repos[i] = models.Repository{ID: instID*1000 + int64(i), Name: n, FullName: org + "/" + n, Owner: models.Owner{ID: orgID, Login: org}, Visibility: "private", Private: true}
	}
	if err := tenancy.UpsertRepositories(ctx, h.pool, instID, repos, time.Now()); err != nil {
		h.t.Fatal(err)
	}
	session := secure.RandomToken()
	if err := identity.CreateSession(ctx, h.pool, identity.NewSession{
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
