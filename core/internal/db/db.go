// Package db owns the PostgreSQL connection pool and schema migrations.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Limits bounds one process's PostgreSQL usage; zero timeouts use server defaults.
type Limits struct {
	MaxConns int32
	// StatementTimeout cancels any single statement that runs longer.
	StatementTimeout time.Duration
	// LockTimeout fails lock waits before they queue later queries.
	LockTimeout time.Duration
	// IdleInTxTimeout releases locks held by idle transactions.
	IdleInTxTimeout time.Duration
}

// Open creates a pool and verifies connectivity.
func Open(ctx context.Context, url string, limits Limits) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	cfg.MaxConns = limits.MaxConns
	cfg.MaxConnIdleTime = 5 * time.Minute
	// Recycle connections so server-side memory growth and changed settings do not persist.
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnLifetimeJitter = 5 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second
	params := cfg.ConnConfig.RuntimeParams
	params["application_name"] = "gitorgasm"
	setTimeout(params, "statement_timeout", limits.StatementTimeout)
	setTimeout(params, "lock_timeout", limits.LockTimeout)
	setTimeout(params, "idle_in_transaction_session_timeout", limits.IdleInTxTimeout)
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, startupWait)
	defer cancel()
	if err := retryUntilReady(pingCtx, pool.Ping, time.Second); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// startupWait bounds how long a starting process waits for PostgreSQL to accept connections.
const startupWait = 30 * time.Second

// retryUntilReady calls ping until it succeeds or ctx ends, and returns the last ping error.
func retryUntilReady(ctx context.Context, ping func(context.Context) error, every time.Duration) error {
	for {
		err := ping(ctx)
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return err
		case <-time.After(every):
		}
	}
}

func setTimeout(params map[string]string, name string, d time.Duration) {
	if d > 0 {
		params[name] = strconv.FormatInt(d.Milliseconds(), 10) + "ms"
	}
}

// Migrate applies goose and River migrations under advisory locks for concurrent runs.
func Migrate(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) error {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sqlDB.Close() }()
	if err := migrateApp(ctx, sqlDB, log); err != nil {
		return err
	}
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("river migrator: %w", err)
	}
	res, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return fmt.Errorf("river migrate: %w", err)
	}
	for _, v := range res.Versions {
		log.Info("applied river migration", "version", v.Version)
	}
	return nil
}

func migrateApp(ctx context.Context, sqlDB *sql.DB, log *slog.Logger) error {
	fsys, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return err
	}
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("goose locker: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, fsys, goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("goose provider: %w", err)
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	for _, r := range results {
		log.Info("applied migration", "source", r.Source.Path, "duration", r.Duration)
	}
	return nil
}
