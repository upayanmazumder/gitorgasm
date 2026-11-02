package administration

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/upayanmazumder/gitorgasm/core/internal/config"
	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
	"github.com/upayanmazumder/gitorgasm/core/internal/secure"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/identity"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/tenancy"
	"github.com/upayanmazumder/gitorgasm/core/internal/testdb"
)

func TestAuthorizerBindsTenantExactSessionAndLiveOrganization(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	sealer, err := secure.NewSealer(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	userID, err := identity.UpsertUser(ctx, pool, &models.User{ID: 42, Login: "owner"})
	if err != nil {
		t.Fatal(err)
	}
	if err := tenancy.UpsertInstallation(ctx, pool, &models.Installation{ID: 7, Account: &models.Account{ID: 100, Login: "acme", Type: "Organization"}, RepositorySelection: "all"}); err != nil {
		t.Fatal(err)
	}
	if err := tenancy.ReplaceUserInstallations(ctx, pool, userID, []int64{7}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	makeSession := func(name, token string, expiry, tokenExpiry time.Time) {
		t.Helper()
		var te *time.Time
		if !tokenExpiry.IsZero() {
			te = &tokenExpiry
		}
		if err := identity.CreateSession(ctx, pool, identity.NewSession{IDHash: secure.Hash(name), UserID: userID, KeyVersion: secure.KeyVersion, TokenSealed: sealer.Seal([]byte(token), []byte(strconv.FormatInt(userID, 10))), ExpiresAt: expiry, TokenExpiresAt: te}); err != nil {
			t.Fatal(err)
		}
	}
	makeSession("actor", "exact-actor-token", now.Add(time.Hour), time.Time{})
	makeSession("latest", "wrong-latest-token", now.Add(2*time.Hour), time.Time{})
	makeSession("expired-token", "expired-secret", now.Add(time.Hour), now.Add(-time.Minute))
	makeSession("expired-session", "expired-secret", now.Add(-time.Minute), time.Time{})
	accountID := int64(100)
	accountLogin, accountType, permission, role, state := "acme", "Organization", "write", "admin", "active"
	membershipID := int64(100)
	suspended, fail := false, 0
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if fail != 0 {
			w.WriteHeader(fail)
			_, _ = w.Write([]byte(`{"message":"private-secret"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/app/installations/7":
			if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ey") {
				t.Error("installation did not use App JWT")
			}
			var suspendedAt *time.Time
			if suspended {
				suspendedAt = &now
			}
			_ = json.NewEncoder(w).Encode(models.Installation{ID: 7, Account: &models.Account{ID: accountID, Login: accountLogin, Type: accountType}, Permissions: map[string]string{"members": permission}, SuspendedAt: suspendedAt})
		case "/user/memberships/orgs/acme":
			if r.Header.Get("Authorization") != "Bearer exact-actor-token" {
				t.Error("not using exact actor session")
			}
			_ = json.NewEncoder(w).Encode(models.Membership{State: state, Role: role, Organization: &models.Account{ID: membershipID, Login: "acme"}, User: &models.User{ID: 42, Login: "owner"}})
		default:
			t.Errorf("unexpected endpoint: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	a := Authorizer{Pool: pool, GitHub: github.New(&config.GitHub{AppID: 1, PrivateKey: key, APIBaseURL: srv.URL, RequestTimeout: time.Second}), Sealer: sealer, Now: func() time.Time { return now }}
	check := func(session, org string) (*Access, error) { return a.Check(ctx, userID, secure.Hash(session), org) }
	assertCode := func(err error, code string, status int) {
		t.Helper()
		var ae *AccessError
		if !errors.As(err, &ae) || ae.Code != code || ae.Status != status || strings.Contains(err.Error(), "secret") {
			t.Fatalf("error=%v want=%s/%d", err, code, status)
		}
	}
	access, err := check("actor", "ACME")
	if err != nil || !access.CanManage || access.Actor.GitHubID != 42 || access.Token != "exact-actor-token" || access.AccountID != 100 || access.InstallationID != 7 {
		t.Fatalf("access=%+v error=%v", access, err)
	}
	encoded, err := json.Marshal(access)
	if err != nil || strings.Contains(string(encoded), "token") || strings.Contains(string(encoded), "SessionHash") {
		t.Fatalf("credential leak: %s %v", encoded, err)
	}
	calls = 0
	_, err = check("actor", "other")
	assertCode(err, "organization_not_found", 404)
	if calls != 0 {
		t.Fatal("cross-tenant request reached GitHub")
	}
	configured := a.GitHub
	a.GitHub = nil
	_, err = check("actor", "other")
	assertCode(err, "organization_not_found", http.StatusNotFound)
	_, err = check("actor", "acme")
	assertCode(err, "github_not_configured", http.StatusServiceUnavailable)
	a.GitHub = configured
	for _, session := range []string{"missing", "expired-session", "expired-token"} {
		calls = 0
		_, err := check(session, "acme")
		if session == "expired-token" {
			assertCode(err, "github_reauthentication_required", 401)
		} else {
			assertCode(err, "session_expired", 401)
		}
		if calls != 0 {
			t.Fatalf("unusable %s reached GitHub", session)
		}
	}
	for _, tc := range []struct {
		name, reason string
		set          func()
		reset        func()
	}{
		{"not owner", "not_owner", func() { role = "member" }, func() { role = "admin" }},
		{"pending owner", "not_owner", func() { state = "pending" }, func() { state = "active" }},
		{"permission", "members_permission_required", func() { permission = "read" }, func() { permission = "write" }},
		{"unsupported", "unsupported_account", func() { accountType = "User" }, func() { accountType = "Organization" }},
		{"suspended", "installation_suspended", func() { suspended = true }, func() { suspended = false }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.set()
			defer tc.reset()
			access, err := check("actor", "acme")
			if err != nil || access.CanManage || access.Reason == nil || *access.Reason != tc.reason {
				t.Fatalf("access=%+v err=%v", access, err)
			}
		})
	}
	accountID = 200
	_, err = check("actor", "acme")
	assertCode(err, "organization_not_found", 404)
	accountID = 100
	accountLogin = "renamed"
	_, err = check("actor", "acme")
	assertCode(err, "organization_not_found", 404)
	accountLogin = "acme"
	membershipID = 200
	_, err = check("actor", "acme")
	assertCode(err, "organization_not_found", 404)
	membershipID = 100
	for _, tc := range []struct {
		status int
		code   string
		result int
	}{{401, "github_reauthentication_required", 401}, {403, "github_forbidden", 403}, {429, "github_rate_limited", 429}, {503, "github_unavailable", 503}} {
		fail = tc.status
		_, err = check("actor", "acme")
		assertCode(err, tc.code, tc.result)
	}
	fail = 0
	if err := tenancy.ReplaceUserInstallations(ctx, pool, userID, nil); err != nil {
		t.Fatal(err)
	}
	calls = 0
	_, err = check("actor", "acme")
	assertCode(err, "organization_not_found", 404)
	if calls != 0 {
		t.Fatal("revoked tenant grant reached GitHub")
	}
}
