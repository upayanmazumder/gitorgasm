package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/upayanmazumder/gitorgasm/core/internal/config"
	"github.com/upayanmazumder/gitorgasm/core/internal/db"
	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs"
	"github.com/upayanmazumder/gitorgasm/core/internal/obs"
	"github.com/upayanmazumder/gitorgasm/core/internal/scorer"
	"github.com/upayanmazumder/gitorgasm/core/internal/secure"
)

func runWorker(ctx context.Context, log *slog.Logger) error {
	cfg, err := config.Load(config.ModeWorker)
	if err != nil {
		return err
	}
	if cfg.GitHub == nil {
		return errors.New("the worker requires the GitHub App configuration (GITHUB_* variables)")
	}
	shutdownTracing, err := obs.InitTracing(ctx, "worker", cfg.OTLPEndpoint)
	if err != nil {
		return err
	}
	pool, err := db.Open(ctx, cfg.DatabaseURL, dbLimits(cfg))
	if err != nil {
		return err
	}
	defer pool.Close()
	sealer, err := secure.NewSealer(cfg.TokenKey)
	if err != nil {
		return err
	}
	client, err := jobs.NewWorkerClient(pool, github.New(cfg.GitHub), jobs.WorkerOptions{
		Concurrency: cfg.WorkerCount, ReconcileEvery: cfg.ReconcileEvery, Logger: log,
		Scorer: scorer.New(cfg.ScoringBin, 1, time.Minute),
		Sealer: sealer,
	})
	if err != nil {
		return err
	}
	if err := client.Start(ctx); err != nil {
		return err
	}
	// The worker has no public port. Its metrics port also answers liveness and readiness.
	metrics := obs.MetricsServer(cfg.MetricsAddr, func(r *http.Request) error { return pool.Ping(r.Context()) })
	return serve(ctx, log, cfg.ShutdownTimeout, []*http.Server{metrics}, func(c context.Context) error {
		return errors.Join(client.Stop(c), shutdownTracing(c))
	})
}
