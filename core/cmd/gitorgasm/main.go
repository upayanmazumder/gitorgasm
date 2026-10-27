// Command gitorgasm runs the API, worker, and migration roles from one image.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/upayanmazumder/gitorgasm/core/internal/config"
	"github.com/upayanmazumder/gitorgasm/core/internal/db"
	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs"
	"github.com/upayanmazumder/gitorgasm/core/internal/obs"
	"github.com/upayanmazumder/gitorgasm/core/internal/scorer"
	"github.com/upayanmazumder/gitorgasm/core/internal/secure"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

// The API runs at most apiScorerSlots score processes and waits briefly for a free slot.
const (
	apiScorerSlots = 2
	apiScorerWait  = 2 * time.Second
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: gitorgasm <api|worker|migrate>")
		os.Exit(2)
	}
	role := os.Args[1]
	log := obs.NewLogger(role).With("app_version", version)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var err error
	switch role {
	case "api":
		err = runAPI(ctx, log)
	case "worker":
		err = runWorker(ctx, log)
	case "migrate":
		err = runMigrate(ctx, log)
	default:
		err = fmt.Errorf("unknown role %q", role)
	}
	if err != nil {
		log.Error("exiting with error", "error", err)
		os.Exit(1)
	}
}

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

func runAPI(ctx context.Context, log *slog.Logger) error {
	cfg, err := config.Load(config.ModeAPI)
	if err != nil {
		return err
	}
	shutdownTracing, err := obs.InitTracing(ctx, "api", cfg.OTLPEndpoint)
	if err != nil {
		return err
	}
	pool, err := db.Open(ctx, cfg.DatabaseURL, dbLimits(cfg))
	if err != nil {
		return err
	}
	defer pool.Close()
	jobClient, err := jobs.NewInsertClient(pool)
	if err != nil {
		return err
	}
	sealer, err := secure.NewSealer(cfg.TokenKey)
	if err != nil {
		return err
	}
	srv := &httpapi.Server{
		Pool: pool, Jobs: jobClient, Sealer: sealer, Cfg: cfg, Log: log, Now: time.Now,
		Scorer: scorer.New(cfg.ScoringBin, apiScorerSlots, apiScorerWait),
	}
	if cfg.GitHub != nil {
		srv.GitHub = github.New(cfg.GitHub)
	} else {
		log.Warn("GitHub App is not configured; auth, sync, and webhook routes return 503")
	}

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
	metrics := obs.MetricsServer(cfg.MetricsAddr, nil)
	return serve(ctx, log, cfg.ShutdownTimeout, []*http.Server{httpServer, metrics}, func(c context.Context) error {
		return shutdownTracing(c)
	})
}

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
	client, err := jobs.NewWorkerClient(pool, github.New(cfg.GitHub), jobs.WorkerOptions{
		Concurrency: cfg.WorkerCount, ReconcileEvery: cfg.ReconcileEvery, Logger: log,
		Scorer: scorer.New(cfg.ScoringBin, 1, time.Minute),
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

// serve runs servers until ctx is cancelled, then drains them within timeout.
func serve(ctx context.Context, log *slog.Logger, timeout time.Duration, servers []*http.Server, onStop func(context.Context) error) error {
	errCh := make(chan error, len(servers))
	for _, s := range servers {
		log.Info("listening", "addr", s.Addr)
		go func() {
			if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- err
			}
		}()
	}
	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-errCh:
	}
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	errs := []error{runErr}
	for _, s := range servers {
		errs = append(errs, s.Shutdown(shutdownCtx))
	}
	errs = append(errs, onStop(shutdownCtx))
	return errors.Join(errs...)
}

func dbLimits(cfg *config.Config) db.Limits {
	return db.Limits{
		MaxConns:         cfg.DBMaxConns,
		StatementTimeout: cfg.DBStatementTimeout,
		LockTimeout:      cfg.DBLockTimeout,
		IdleInTxTimeout:  cfg.DBIdleInTxTimeout,
	}
}
