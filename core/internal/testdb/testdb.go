// Package testdb creates isolated migrated databases; CI requires TEST_DATABASE_URL so tests cannot silently skip.
package testdb

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/upayanmazumder/gitorgasm/core/internal/db"
	"github.com/upayanmazumder/gitorgasm/core/internal/secure"
)

// New creates a database, applies all migrations, and drops it when the test ends.
func New(t testing.TB) *pgxpool.Pool {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("TEST_DATABASE_URL is required in CI")
		}
		t.Skip("TEST_DATABASE_URL is not set; run `make test`")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	name := "gitorgasm_test_" + strings.ToLower(strings.NewReplacer("-", "", "_", "").Replace(secure.RandomToken()[:12]))
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	u.Path = "/" + name
	pool, err := db.Open(ctx, u.String(), db.Limits{
		MaxConns: 8, StatementTimeout: 15 * time.Second, LockTimeout: 5 * time.Second, IdleInTxTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := db.Migrate(ctx, pool, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		cctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := admin.Exec(cctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", name)); err != nil {
			t.Logf("drop database %s: %v", name, err)
		}
		_ = admin.Close(cctx)
	})
	return pool
}
