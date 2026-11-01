package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/upayanmazumder/gitorgasm/core/internal/config"
	"github.com/upayanmazumder/gitorgasm/core/internal/db"
)

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
