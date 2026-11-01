package jobs_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/config"
	"github.com/upayanmazumder/gitorgasm/core/internal/github"
	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/queue"
	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/sync"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/tenancy"
	"github.com/upayanmazumder/gitorgasm/core/internal/testdb"
)

func setup(t *testing.T, f *fakeGitHub) (*pgxpool.Pool, *github.Client) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f.key = key
	srv := httptest.NewServer(f.handler(t))
	t.Cleanup(srv.Close)
	gh := github.New(&config.GitHub{AppID: 42, PrivateKey: key, APIBaseURL: srv.URL, WebBaseURL: srv.URL, RequestTimeout: 5 * time.Second})
	return testdb.New(t), gh
}

func activeRepos(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT name FROM repositories WHERE deleted_at IS NULL ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return names
}

func seedStale(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	inst := &models.Installation{ID: instID, RepositorySelection: "all", Account: &models.Account{ID: 100, Login: "acme", Type: "Organization"}}
	if err := tenancy.UpsertInstallation(ctx, pool, inst); err != nil {
		t.Fatal(err)
	}
	stale := []models.Repository{{ID: 999, Name: "removed", FullName: "acme/removed", Owner: models.Owner{ID: 100}}}
	if err := tenancy.UpsertRepositories(ctx, pool, instID, stale, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
}

func work(t *testing.T, w *sync.Worker) error {
	t.Helper()
	return w.Work(context.Background(), &river.Job[queue.SyncInstallation]{Args: queue.SyncInstallation{InstallationID: instID}})
}
