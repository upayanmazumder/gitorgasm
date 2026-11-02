package config

import (
	"encoding/base64"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestWorkerPoolIncludesAdministrationLockAndAuthorizationHeadroom(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range githubKeys {
		t.Setenv(key, "")
	}
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("TOKEN_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	t.Setenv("SCORING_BIN", binary)
	for _, workers := range []int{1, 4, 10} {
		t.Run(strconv.Itoa(workers), func(t *testing.T) {
			t.Setenv("WORKER_CONCURRENCY", strconv.Itoa(workers))
			t.Setenv("DB_MAX_CONNS", "")
			cfg, err := Load(ModeWorker)
			if err != nil {
				t.Fatal(err)
			}
			want := 2*workers + 1 + 2 + riverConns
			if int(cfg.DBMaxConns) != want {
				t.Fatalf("worker pool = %d; want %d", cfg.DBMaxConns, want)
			}
			t.Setenv("DB_MAX_CONNS", strconv.Itoa(want-1))
			if _, err := Load(ModeWorker); err == nil || !strings.Contains(err.Error(), "2 administration connections") {
				t.Fatalf("undersized worker pool accepted: %v", err)
			}
			t.Setenv("DB_MAX_CONNS", strconv.Itoa(want))
			if _, err := Load(ModeWorker); err != nil {
				t.Fatalf("minimum worker pool rejected: %v", err)
			}
		})
	}
}
