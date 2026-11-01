package jobs_test

import (
	"context"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertest"

	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/queue"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/webhook"
	"github.com/upayanmazumder/gitorgasm/core/internal/secure"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/identity"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/tenancy"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/webhooks"
)

func TestWebhookWorkerAppliesEvents(t *testing.T) {
	f := &fakeGitHub{}
	pool, gh := setup(t, f)
	ctx := context.Background()
	seedStale(t, pool)
	userID, err := identity.UpsertUser(ctx, pool, &models.User{ID: 7, Login: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if err := tenancy.ReplaceUserInstallations(ctx, pool, userID, []int64{instID}); err != nil {
		t.Fatal(err)
	}
	if err := identity.CreateSession(ctx, pool, identity.NewSession{IDHash: secure.Hash("h"), UserID: userID, KeyVersion: 1, TokenSealed: []byte{1}, ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}

	tw := rivertest.NewWorker(t, riverpgxv5.New(pool), &river.Config{}, &webhook.Worker{Pool: pool, GitHub: gh})
	deliver := func(id, event, payload string) {
		t.Helper()
		if _, err := webhooks.InsertWebhookDelivery(ctx, pool, webhooks.WebhookDelivery{DeliveryID: id, Event: event, Payload: []byte(payload)}); err != nil {
			t.Fatal(err)
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		res, err := tw.Work(ctx, t, tx, queue.ProcessWebhook{DeliveryID: id}, nil)
		if err != nil || res.EventKind != river.EventKindJobCompleted {
			t.Fatalf("work %s: %v %+v", id, err, res)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}

	deliver("d-revoke", "github_app_authorization", `{"action":"revoked","sender":{"id":7}}`)
	var sessions, grants int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM sessions`).Scan(&sessions)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM user_installations`).Scan(&grants)
	if sessions != 0 || grants != 0 {
		t.Fatalf("after revoke sessions=%d grants=%d, want 0 and 0", sessions, grants)
	}

	// Restore the grant (the user signed in again), then uninstall the App.
	if err := tenancy.ReplaceUserInstallations(ctx, pool, userID, []int64{instID}); err != nil {
		t.Fatal(err)
	}
	deliver("d-uninstall", "installation", `{"action":"deleted","installation":{"id":10,"repository_selection":"all","account":{"id":100,"login":"acme","type":"Organization"}}}`)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM user_installations`).Scan(&grants)
	if grants != 0 {
		t.Fatalf("grants after uninstall = %d, want 0", grants)
	}
	var processed int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM webhook_deliveries WHERE processed_at IS NOT NULL`).Scan(&processed)
	if processed != 2 {
		t.Fatalf("processed deliveries = %d, want 2", processed)
	}
}
