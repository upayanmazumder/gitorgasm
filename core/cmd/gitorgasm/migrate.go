package main

import (
	"context"
	"log/slog"

	"github.com/upayanmazumder/gitorgasm/core/internal/config"
	"github.com/upayanmazumder/gitorgasm/core/internal/db"
)

func runMigrate(ctx context.Context, log *slog.Logger) error {
	cfg, err := config.Load(config.ModeMigrate)
	if err != nil {
		return err
	}
	// Migrations may run long, but lock waits must fail before blocking live traffic.
	pool, err := db.Open(ctx, cfg.DatabaseURL, db.Limits{MaxConns: 2, LockTimeout: cfg.DBLockTimeout})
	if err != nil {
		return err
	}
	defer pool.Close()
	return db.Migrate(ctx, pool, log)
}
