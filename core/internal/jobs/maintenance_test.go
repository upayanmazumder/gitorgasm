package jobs_test

import (
	"context"
	"encoding/hex"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/maintenance"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/queue"
	"github.com/upayanmazumder/gitorgasm/core/internal/secure"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/identity"
	"github.com/upayanmazumder/gitorgasm/core/internal/testdb"
)

func TestPurgeKeepsRowsInsideRetention(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	now := time.Now()
	day := 24 * time.Hour
	userID, err := identity.UpsertUser(ctx, pool, &models.User{ID: 7, Login: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	for id, expires := range map[string]time.Time{"expired": now.Add(-time.Minute), "live": now.Add(time.Hour)} {
		if err := identity.CreateSession(ctx, pool, identity.NewSession{IDHash: secure.Hash(id), UserID: userID, KeyVersion: 1, TokenSealed: []byte{1}, ExpiresAt: expires}); err != nil {
			t.Fatal(err)
		}
	}
	deliveries := []struct {
		id        string
		age       time.Duration
		processed bool
	}{
		{"processed-old", 15 * day, true},
		{"processed-recent", 13 * day, true},
		{"unprocessed-old", 31 * day, false},
		{"unprocessed-recent", 20 * day, false},
	}
	for _, d := range deliveries {
		if _, err := pool.Exec(ctx, `
			INSERT INTO webhook_deliveries (delivery_id, event, payload, received_at, processed_at)
			VALUES ($1, 'ping', '{}', $2::timestamptz, CASE WHEN $3::boolean THEN $2::timestamptz END)`, d.id, now.Add(-d.age), d.processed); err != nil {
			t.Fatal(err)
		}
	}

	log := slog.New(slog.DiscardHandler)
	clock := func() time.Time { return now }
	if err := (&maintenance.PurgeSessionsWorker{Pool: pool, Now: clock, Log: log}).Work(ctx, &river.Job[queue.PurgeSessions]{}); err != nil {
		t.Fatal(err)
	}
	if err := (&maintenance.PurgeWebhookDeliveriesWorker{Pool: pool, Now: clock, Log: log}).Work(ctx, &river.Job[queue.PurgeWebhookDeliveries]{}); err != nil {
		t.Fatal(err)
	}

	ids := func(sql string) string {
		t.Helper()
		rows, err := pool.Query(ctx, sql)
		if err != nil {
			t.Fatal(err)
		}
		got, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			t.Fatal(err)
		}
		return strings.Join(got, ",")
	}
	if got, want := ids(`SELECT encode(id_hash, 'hex') FROM sessions`), hex.EncodeToString(secure.Hash("live")); got != want {
		t.Fatalf("sessions = %q, want only the live session %q", got, want)
	}
	if got := ids(`SELECT delivery_id FROM webhook_deliveries ORDER BY 1`); got != "processed-recent,unprocessed-recent" {
		t.Fatalf("deliveries = %q, want processed-recent,unprocessed-recent", got)
	}
}
