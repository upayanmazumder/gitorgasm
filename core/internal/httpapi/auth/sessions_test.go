package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/upayanmazumder/gitorgasm/core/internal/config"
	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
	"github.com/upayanmazumder/gitorgasm/core/internal/secure"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/identity"
	"github.com/upayanmazumder/gitorgasm/core/internal/testdb"
)

func TestRequireSessionBindsExactCookieHashWithoutExposingIt(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	userID, err := identity.UpsertUser(ctx, pool, &models.User{ID: 42, Login: "owner"})
	if err != nil {
		t.Fatal(err)
	}
	for _, cookie := range []string{"first-cookie", "second-cookie"} {
		if err := identity.CreateSession(ctx, pool, identity.NewSession{IDHash: secure.Hash(cookie), UserID: userID, KeyVersion: secure.KeyVersion, TokenSealed: []byte("sealed"), ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
			t.Fatal(err)
		}
	}
	h := Handler{Pool: pool, Cfg: &config.Config{}, Log: slog.New(slog.DiscardHandler)}
	for _, cookie := range []string{"first-cookie", "second-cookie"} {
		called := false
		handler := h.RequireSession(func(w http.ResponseWriter, r *http.Request, u *identity.SessionUser) {
			called = true
			if !bytes.Equal(u.SessionHash, secure.Hash(cookie)) || u.UserID != userID {
				t.Fatal("principal is not bound to the exact cookie")
			}
			_ = json.NewEncoder(w).Encode(u)
		})
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: cookie})
		rec := httptest.NewRecorder()
		handler(rec, req)
		if !called || rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "SessionHash") || strings.Contains(rec.Body.String(), "session_hash") || strings.Contains(rec.Body.String(), cookie) {
			t.Fatalf("unsafe principal: %d %s", rec.Code, rec.Body)
		}
	}
	if err := identity.DeleteSession(ctx, pool, secure.Hash("first-cookie")); err != nil {
		t.Fatal(err)
	}
	called := false
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "first-cookie"})
	rec := httptest.NewRecorder()
	h.RequireSession(func(http.ResponseWriter, *http.Request, *identity.SessionUser) { called = true })(rec, req)
	if called || rec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session admitted: %d", rec.Code)
	}
}
