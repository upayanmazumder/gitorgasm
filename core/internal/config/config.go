// Package config loads and validates process configuration from environment variables.
package config

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// GitHub holds GitHub App settings. The integration is enabled only when every field is set.
type GitHub struct {
	AppID          int64
	ClientID       string
	ClientSecret   string
	PrivateKey     *rsa.PrivateKey
	WebhookSecret  []byte
	APIBaseURL     string // https://api.github.com
	WebBaseURL     string // https://github.com
	OAuthCallback  string // public URL of /v1/auth/callback
	InstallURL     string // https://github.com/apps/<slug>/installations/new
	RequestTimeout time.Duration
}

// Config is the full validated configuration.
type Config struct {
	Env                string
	HTTPAddr           string
	MetricsAddr        string
	DatabaseURL        string
	DBMaxConns         int32
	DBStatementTimeout time.Duration // cancels one statement that runs longer
	DBLockTimeout      time.Duration // fails a statement that waits longer for a lock
	DBIdleInTxTimeout  time.Duration // ends sessions idle inside an open transaction
	AppOrigin          *url.URL      // frontend origin used for redirects and Origin checks
	CookieSecure       bool
	SessionTTL         time.Duration
	TokenKey           []byte // 32-byte AES-256-GCM key for GitHub tokens at rest
	OTLPEndpoint       string // empty disables trace export
	WorkerCount        int
	ReconcileEvery     time.Duration
	ShutdownTimeout    time.Duration
	GitHub             *GitHub // nil when the integration is not configured
}

// Mode selects which settings are required.
type Mode int

const (
	ModeAPI Mode = iota
	ModeWorker
	ModeMigrate
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
		errs = append(errs, fmt.Errorf("DB_MAX_CONNS must be at least %d for WORKER_CONCURRENCY=%d (2 queues x workers + %d for River)",
			minWorkerConns(workers), workers, riverConns))
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

// minWorkerConns reserves one connection per job across both queues plus River headroom.
func minWorkerConns(workers int) int { return 2*workers + riverConns }

var githubKeys = []string{
	"GITHUB_APP_ID", "GITHUB_CLIENT_ID", "GITHUB_CLIENT_SECRET", "GITHUB_PRIVATE_KEY",
	"GITHUB_WEBHOOK_SECRET", "GITHUB_OAUTH_CALLBACK_URL", "GITHUB_APP_SLUG",
}

// loadGitHub returns nil when no GitHub variable is set. Partial configuration is an error.
func loadGitHub(get func(string, string) string) (*GitHub, []error) {
	var set, missing []string
	for _, k := range githubKeys {
		if get(k, "") != "" {
			set = append(set, k)
		} else {
			missing = append(missing, k)
		}
	}
	if len(set) == 0 {
		return nil, nil
	}
	if len(missing) > 0 {
		return nil, []error{fmt.Errorf("GitHub App configuration is partial; missing %s", strings.Join(missing, ", "))}
	}
	var errs []error
	appID, err := strconv.ParseInt(get("GITHUB_APP_ID", ""), 10, 64)
	if err != nil || appID <= 0 {
		errs = append(errs, errors.New("GITHUB_APP_ID must be a positive integer"))
	}
	pk, err := parsePrivateKey(get("GITHUB_PRIVATE_KEY", ""))
	if err != nil {
		errs = append(errs, fmt.Errorf("GITHUB_PRIVATE_KEY: %w", err))
	}
	timeout, err := time.ParseDuration(get("GITHUB_REQUEST_TIMEOUT", "20s"))
	if err != nil || timeout <= 0 {
		errs = append(errs, errors.New("GITHUB_REQUEST_TIMEOUT must be a positive duration"))
	}
	web := strings.TrimRight(get("GITHUB_WEB_BASE_URL", "https://github.com"), "/")
	return &GitHub{
		AppID:          appID,
		ClientID:       get("GITHUB_CLIENT_ID", ""),
		ClientSecret:   get("GITHUB_CLIENT_SECRET", ""),
		PrivateKey:     pk,
		WebhookSecret:  []byte(get("GITHUB_WEBHOOK_SECRET", "")),
		APIBaseURL:     strings.TrimRight(get("GITHUB_API_BASE_URL", "https://api.github.com"), "/"),
		WebBaseURL:     web,
		OAuthCallback:  get("GITHUB_OAUTH_CALLBACK_URL", ""),
		InstallURL:     web + "/apps/" + get("GITHUB_APP_SLUG", "") + "/installations/new",
		RequestTimeout: timeout,
	}, errs
}

// parsePrivateKey accepts PEM text, PEM with literal "\n" escapes, or base64-encoded PEM.
func parsePrivateKey(raw string) (*rsa.PrivateKey, error) {
	text := strings.ReplaceAll(raw, `\n`, "\n")
	if !strings.Contains(text, "-----BEGIN") {
		dec, err := base64.StdEncoding.DecodeString(strings.TrimSpace(text))
		if err != nil {
			return nil, errors.New("not PEM and not base64-encoded PEM")
		}
		text = string(dec)
	}
	block, _ := pem.Decode([]byte(text))
	if block == nil {
		return nil, errors.New("no PEM block found")
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("unsupported private key encoding")
	}
	rk, ok := k.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not RSA")
	}
	return rk, nil
}
