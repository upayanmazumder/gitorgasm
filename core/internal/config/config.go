// Package config loads and validates process configuration from environment variables.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"
)

// Load reads configuration for the given mode and returns every validation problem at once.
func Load(mode Mode) (*Config, error) {
	var errs []error
	get := func(k, def string) string {
		if v, ok := os.LookupEnv(k); ok && v != "" {
			return v
		}
		return def
	}
	req := func(k string) string {
		v := os.Getenv(k)
		if v == "" {
			errs = append(errs, fmt.Errorf("%s is required", k))
		}
		return v
	}
	dur := func(k, def string) time.Duration {
		d, err := time.ParseDuration(get(k, def))
		if err != nil || d <= 0 {
			errs = append(errs, fmt.Errorf("%s must be a positive duration", k))
		}
		return d
	}
	integer := func(k string, def int) int {
		n, err := strconv.Atoi(get(k, strconv.Itoa(def)))
		if err != nil || n <= 0 {
			errs = append(errs, fmt.Errorf("%s must be a positive integer", k))
		}
		return n
	}

	workers := integer("WORKER_CONCURRENCY", 4)
	defaultConns := 10
	if mode == ModeWorker {
		defaultConns = minWorkerConns(workers)
	}
	c := &Config{
		Env:                get("GITORGASM_ENV", "development"),
		HTTPAddr:           get("HTTP_ADDR", ":8080"),
		MetricsAddr:        get("METRICS_ADDR", ":9090"),
		DatabaseURL:        req("DATABASE_URL"),
		DBMaxConns:         int32(min(integer("DB_MAX_CONNS", defaultConns), 1000)), //nolint:gosec // clamped to 1000 above
		DBStatementTimeout: dur("DB_STATEMENT_TIMEOUT", "15s"),
		DBLockTimeout:      dur("DB_LOCK_TIMEOUT", "5s"),
		DBIdleInTxTimeout:  dur("DB_IDLE_IN_TX_TIMEOUT", "30s"),
		CookieSecure:       get("COOKIE_SECURE", "true") != "false",
		SessionTTL:         dur("SESSION_TTL", "168h"),
		OTLPEndpoint:       get("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		WorkerCount:        workers,
		ReconcileEvery:     dur("RECONCILE_INTERVAL", "6h"),
		ShutdownTimeout:    dur("SHUTDOWN_TIMEOUT", "25s"),
	}
	if mode == ModeMigrate {
		return c, errors.Join(errs...)
	}
	if mode == ModeWorker && workers > 0 && int(c.DBMaxConns) < minWorkerConns(workers) {
		errs = append(errs, fmt.Errorf("DB_MAX_CONNS must be at least %d for WORKER_CONCURRENCY=%d (2 queues x workers + 1 scoring job + 2 administration connections + %d for River)",
			minWorkerConns(workers), workers, riverConns))
	}
	c.ScoringBin = get("SCORING_BIN", "/gitorgasm-score")
	if info, err := os.Stat(c.ScoringBin); err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		errs = append(errs, fmt.Errorf("SCORING_BIN must name an executable file, got %q", c.ScoringBin))
	}

	if mode == ModeAPI {
		origin, err := url.Parse(req("APP_ORIGIN"))
		if err != nil || origin.Scheme == "" || origin.Host == "" {
			errs = append(errs, errors.New("APP_ORIGIN must be an absolute URL such as https://gitorgasm.upayan.dev"))
		}
		c.AppOrigin = origin
	}
	key, err := base64.StdEncoding.DecodeString(req("TOKEN_ENCRYPTION_KEY"))
	if err != nil || len(key) != 32 {
		errs = append(errs, errors.New("TOKEN_ENCRYPTION_KEY must be 32 bytes, base64-encoded"))
	}
	c.TokenKey = key

	gh, ghErrs := loadGitHub(get)
	errs = append(errs, ghErrs...)
	c.GitHub = gh
	return c, errors.Join(errs...)
}

// riverConns reserves connections for River's listener, fetching, completion, election, and maintenance.
const riverConns = 4

// minWorkerConns reserves both ordinary queues, scoring, an administration lock plus authorization, and River headroom.
func minWorkerConns(workers int) int { return 2*workers + 3 + riverConns }
