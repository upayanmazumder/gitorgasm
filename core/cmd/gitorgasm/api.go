package main

import (
	"context"
	"log/slog"
	"net/http"
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

const (
	apiScorerSlots = 2
	apiScorerWait  = 2 * time.Second
)

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
