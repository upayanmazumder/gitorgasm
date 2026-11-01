package config

import (
	"crypto/rsa"
	"net/url"
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
	ScoringBin         string  // Rust score binary run by the api and worker
	GitHub             *GitHub // nil when the integration is not configured
}

// Mode selects which settings are required.
type Mode int

const (
	ModeAPI Mode = iota
	ModeWorker
	ModeMigrate
)
