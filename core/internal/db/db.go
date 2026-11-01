// Package db owns the PostgreSQL connection pool and schema migrations.
package db

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

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
	if err := retryUntilReady(pingCtx, pool.Ping, transientStartupError, time.Second); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

func setTimeout(params map[string]string, name string, d time.Duration) {
	if d > 0 {
		params[name] = strconv.FormatInt(d.Milliseconds(), 10) + "ms"
	}
}
