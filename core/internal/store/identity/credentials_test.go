package identity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
	"github.com/upayanmazumder/gitorgasm/core/internal/secure"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
	"github.com/upayanmazumder/gitorgasm/core/internal/testdb"
)

func TestLookupSessionCredentialRequiresExactActorSessionAndExpiry(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	userID, err := UpsertUser(ctx, pool, &models.User{ID: 1, Login: "actor"})
	if err != nil {
		t.Fatal(err)
	}
	otherID, err := UpsertUser(ctx, pool, &models.User{ID: 2, Login: "other"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	tokenExpiry := now.Add(time.Hour)
	for _, name := range []string{"exact", "newer"} {
		if err := CreateSession(ctx, pool, NewSession{IDHash: secure.Hash(name), UserID: userID, KeyVersion: secure.KeyVersion, TokenSealed: []byte(name + "-secret"), TokenExpiresAt: &tokenExpiry, ExpiresAt: now.Add(time.Hour)}); err != nil {
			t.Fatal(err)
		}
	}
	credential, err := LookupSessionCredential(ctx, pool, userID, secure.Hash("exact"), now)
	if err != nil || !bytes.Equal(credential.TokenSealed, []byte("exact-secret")) || credential.GitHubID != 1 || credential.TokenExpiresAt == nil {
		t.Fatalf("wrong credential: %+v %v", credential, err)
	}
	encoded, err := json.Marshal(credential)
	if err != nil || strings.Contains(string(encoded), "secret") || string(encoded) != "{}" {
		t.Fatalf("credential can leak: %s %v", encoded, err)
	}
	for _, test := range []struct {
		user int64
		hash []byte
		at   time.Time
	}{
		{otherID, secure.Hash("exact"), now},
		{userID, secure.Hash("missing"), now},
		{userID, secure.Hash("exact"), now.Add(2 * time.Hour)},
	} {
		if _, err := LookupSessionCredential(ctx, pool, test.user, test.hash, test.at); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("admitted wrong actor/session/expiry: %v", err)
		}
	}
	if err := DeleteSession(ctx, pool, secure.Hash("exact")); err != nil {
		t.Fatal(err)
	}
	if _, err := LookupSessionCredential(ctx, pool, userID, secure.Hash("exact"), now); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("fell back to another session: %v", err)
	}
}
