package db

import (
	"context"
	"io"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"github.com/upayanmazumder/gitorgasm/core/internal/secure"
)

// Production recorded version 1 before the schema guards and non-unique login index existed.
func TestUpgradeFromDeployedVersionOne(t *testing.T) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("TEST_DATABASE_URL is required in CI")
		}
		t.Skip("TEST_DATABASE_URL is not set; run make test")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	name := "gitorgasm_upgrade_" + strings.ToLower(strings.ReplaceAll(secure.RandomToken()[:12], "-", "_"))
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := admin.Exec(cleanupCtx, "DROP DATABASE "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("drop test database: %v", err)
		}
		_ = admin.Close(cleanupCtx)
	})
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	pool, err := Open(ctx, u.String(), Limits{MaxConns: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sqlDB.Close() }()
	files, err := fs.Sub(migrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, files, goose.WithSessionLocker(locker))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 1); err != nil {
		t.Fatalf("apply deployed migration: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO accounts (github_id, login, type) VALUES (101, 'old-login', 'Organization')`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, pool, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatalf("upgrade deployed schema: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO accounts (github_id, login, type) VALUES (102, 'old-login', 'Organization')`); err != nil {
		t.Fatalf("login reuse after upgrade: %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM accounts WHERE login = 'old-login'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("want both GitHub accounts after upgrade, got %d", count)
	}
}
