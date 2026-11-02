package administration_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/upayanmazumder/gitorgasm/core/internal/jobs/queue"
	"github.com/upayanmazumder/gitorgasm/core/internal/store"
	adminstore "github.com/upayanmazumder/gitorgasm/core/internal/store/administration"
	"github.com/upayanmazumder/gitorgasm/core/internal/testdb"
)

type fixture struct {
	pool  *pgxpool.Pool
	actor int64
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	f := fixture{pool: testdb.New(t)}
	ctx := context.Background()
	if err := f.pool.QueryRow(ctx, `INSERT INTO users (github_id, login) VALUES (1, 'owner') RETURNING id`).Scan(&f.actor); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO accounts (github_id, login, type) VALUES (100, 'acme', 'Organization'), (200, 'globex', 'Organization')`,
		`INSERT INTO installations (github_id, account_id, repository_selection) VALUES (10, 100, 'all'), (20, 200, 'all')`,
	} {
		f.exec(t, statement)
	}
	f.exec(t, `INSERT INTO user_installations (user_id, installation_id) VALUES ($1, 10), ($1, 20)`, f.actor)
	return f
}

func (f fixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func (f fixture) input(n int) adminstore.NewOperation {
	hash := sha256.Sum256([]byte("session"))
	fingerprint := sha256.Sum256([]byte("invite:First,Second,Third"))
	return adminstore.NewOperation{
		ID: fmt.Sprintf("receipt-%03d", n), InstallationID: 10, AccountID: 100, ActorUserID: f.actor,
		SessionHash: hash[:], IdempotencyKey: fmt.Sprintf("00000000-0000-4000-8000-%012d", n),
		Fingerprint: fingerprint[:], Kind: "invite", Logins: []string{"First", "Second", "Third"},
	}
}

func (f fixture) insert(t *testing.T, n int, enqueue bool) adminstore.Operation {
	t.Helper()
	ctx := context.Background()
	var op adminstore.Operation
	err := pgx.BeginFunc(ctx, f.pool, func(tx pgx.Tx) error {
		var created bool
		var err error
		op, created, err = adminstore.InsertOperation(ctx, tx, f.input(n))
		if err != nil {
			return err
		}
		if !created {
			return errors.New("fixture operation unexpectedly replayed")
		}
		if enqueue {
			client, err := queue.NewInsertClient(f.pool)
			if err != nil {
				return err
			}
			job, err := client.InsertTx(ctx, tx, queue.AdministerOrganization{OperationID: op.ID}, nil)
			if err != nil {
				return err
			}
			return adminstore.SetJobID(ctx, tx, op.ID, job.Job.ID)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return op
}

func TestInsertOperationConcurrentReplayAndConflict(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	type result struct {
		op      adminstore.Operation
		created bool
		err     error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	for i := range 2 {
		go func() {
			<-start
			in := f.input(1)
			in.ID = fmt.Sprintf("concurrent-%d", i)
			var r result
			r.err = pgx.BeginFunc(ctx, f.pool, func(tx pgx.Tx) error {
				var err error
				r.op, r.created, err = adminstore.InsertOperation(ctx, tx, in)
				return err
			})
			results <- r
		}()
	}
	close(start)
	first, second := <-results, <-results
	if first.err != nil || second.err != nil || first.op.ID != second.op.ID || first.created == second.created {
		t.Fatalf("concurrent results: %+v %+v", first, second)
	}
	if len(first.op.Results) != 3 || first.op.Results[0].Login != "First" || first.op.Results[2].Login != "Third" {
		t.Fatalf("ordered targets = %+v", first.op.Results)
	}
	in := f.input(1)
	in.Fingerprint = make([]byte, 32)
	err := pgx.BeginFunc(ctx, f.pool, func(tx pgx.Tx) error {
		_, _, err := adminstore.InsertOperation(ctx, tx, in)
		return err
	})
	if !errors.Is(err, adminstore.ErrIdempotencyConflict) {
		t.Fatalf("changed payload = %v", err)
	}
	var targets int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM administration_targets`).Scan(&targets); err != nil || targets != 3 {
		t.Fatalf("target count = %d (%v)", targets, err)
	}
}

func TestOperationActorTenantAndGrantIsolation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	op := f.insert(t, 1, false)
	for _, scope := range [][2]int64{{f.actor + 1, 10}, {f.actor, 20}} {
		if _, err := adminstore.GetOperation(ctx, f.pool, scope[0], scope[1], op.ID); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("scope %v read = %v", scope, err)
		}
		list, err := adminstore.ListOperations(ctx, f.pool, scope[0], scope[1], nil, 11)
		if err != nil || len(list) != 0 {
			t.Fatalf("scope %v list = %+v (%v)", scope, list, err)
		}
	}
	f.exec(t, `DELETE FROM user_installations WHERE user_id = $1 AND installation_id = 10`, f.actor)
	if _, err := adminstore.GetOperation(ctx, f.pool, f.actor, 10, op.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("revoked read = %v", err)
	}
	if _, err := adminstore.LoadExecution(ctx, f.pool, op.ID); err != nil {
		t.Fatalf("revoked grant must not prevent worker finalization: %v", err)
	}
	err := pgx.BeginFunc(ctx, f.pool, func(tx pgx.Tx) error {
		_, _, err := adminstore.InsertOperation(ctx, tx, f.input(2))
		return err
	})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("revoked insert = %v", err)
	}
}

func TestOperationInsertRollbackAndKeyScope(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	stop := errors.New("rollback")
	err := pgx.BeginFunc(ctx, f.pool, func(tx pgx.Tx) error {
		if _, _, err := adminstore.InsertOperation(ctx, tx, f.input(1)); err != nil {
			return err
		}
		return stop
	})
	if !errors.Is(err, stop) {
		t.Fatal(err)
	}
	var count int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM administration_operations`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rollback rows = %d (%v)", count, err)
	}
	f.insert(t, 1, false)
	in := f.input(1)
	in.ID, in.InstallationID, in.AccountID = "other-tenant", 20, 200
	err = pgx.BeginFunc(ctx, f.pool, func(tx pgx.Tx) error {
		_, created, err := adminstore.InsertOperation(ctx, tx, in)
		if err == nil && !created {
			return errors.New("key incorrectly shared across tenants")
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCompletedOperationRetainsResultsWithoutRiverJob(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	op := f.insert(t, 1, false)
	if claimed, err := adminstore.ClaimOperation(ctx, f.pool, op.ID); err != nil || !claimed {
		t.Fatalf("claim = %v (%v)", claimed, err)
	}
	if err := adminstore.FinishTarget(ctx, f.pool, op.ID, 0, "failed", "upstream-token private@example.com"); err == nil {
		t.Fatal("accepted an untrusted result code")
	}
	for position := range 3 {
		if err := adminstore.StartTarget(ctx, f.pool, op.ID, position); err != nil {
			t.Fatal(err)
		}
		if err := adminstore.FinishTarget(ctx, f.pool, op.ID, position, "invited", "invitation_sent"); err != nil {
			t.Fatal(err)
		}
	}
	stop := errors.New("rollback finalization")
	err := pgx.BeginFunc(ctx, f.pool, func(tx pgx.Tx) error {
		if err := adminstore.FinalizeOperation(ctx, tx, op.ID, "completed", ""); err != nil {
			return err
		}
		return stop
	})
	if !errors.Is(err, stop) {
		t.Fatal(err)
	}
	stored, err := adminstore.LoadExecution(ctx, f.pool, op.ID)
	if err != nil || stored.Status != "running" || stored.CompletedAt != nil {
		t.Fatalf("rolled back finalization = %+v (%v)", stored, err)
	}
	if err := adminstore.FinalizeOperation(ctx, f.pool, op.ID, "completed", ""); err != nil {
		t.Fatal(err)
	}
	view, err := adminstore.GetOperation(ctx, f.pool, f.actor, 10, op.ID)
	if err != nil || view.Status != "completed" || view.CompletedAt == nil || len(view.Results) != 3 {
		t.Fatalf("completed receipt = %+v (%v)", view, err)
	}
	for _, target := range view.Results {
		if target.Status != "invited" || target.Code == nil || *target.Code != "invitation_sent" || target.Message == nil {
			t.Fatalf("finished target changed = %+v", target)
		}
	}
}

func TestExecutionTransitionsPreserveFinishedResults(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	op := f.insert(t, 1, false)
	for i, want := range []bool{true, false} {
		claimed, err := adminstore.ClaimOperation(ctx, f.pool, op.ID)
		if err != nil || claimed != want {
			t.Fatalf("claim %d = %v (%v)", i, claimed, err)
		}
	}
	if err := adminstore.FinishTarget(ctx, f.pool, op.ID, 0, "unchanged", "already_member"); err != nil {
		t.Fatal(err)
	}
	if err := adminstore.StartTarget(ctx, f.pool, op.ID, 1); err != nil {
		t.Fatal(err)
	}
	if err := adminstore.StartTarget(ctx, f.pool, op.ID, 1); err == nil {
		t.Fatal("running target was started twice")
	}
	if err := adminstore.FinalizeOperation(ctx, f.pool, op.ID, "completed", ""); err == nil {
		t.Fatal("completed operation with unfinished targets")
	}
	if err := adminstore.FinalizeOperation(ctx, f.pool, op.ID, "interrupted", "not_owner"); err != nil {
		t.Fatal(err)
	}
	if err := adminstore.FinishTarget(ctx, f.pool, op.ID, 0, "failed", "github_forbidden"); err == nil {
		t.Fatal("terminal target was overwritten")
	}
	execution, err := adminstore.LoadExecution(ctx, f.pool, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	if execution.Status != "interrupted" || execution.CompletedAt == nil || execution.Results[0].Status != "unchanged" ||
		execution.Results[1].Status != "unknown" || *execution.Results[1].Code != "outcome_unknown" ||
		execution.Results[2].Status != "not_attempted" || *execution.Results[2].Code != "not_owner" {
		t.Fatalf("interrupted execution = %+v", execution)
	}
	if err := adminstore.FinalizeOperation(ctx, f.pool, op.ID, "interrupted", "interrupted"); err != nil {
		t.Fatalf("idempotent finalization = %v", err)
	}
}

func TestChangedMembershipOutcomeStopsWithoutOverwritingUnknownResult(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	op := f.insert(t, 1, false)
	if claimed, err := adminstore.ClaimOperation(ctx, f.pool, op.ID); err != nil || !claimed {
		t.Fatalf("claim = %v (%v)", claimed, err)
	}
	if err := adminstore.StartTarget(ctx, f.pool, op.ID, 0); err != nil {
		t.Fatal(err)
	}
	if err := adminstore.FinishTarget(ctx, f.pool, op.ID, 0, "unknown", "membership_changed"); err != nil {
		t.Fatal(err)
	}
	if err := adminstore.FinalizeOperation(ctx, f.pool, op.ID, "interrupted", "membership_changed"); err != nil {
		t.Fatal(err)
	}
	view, err := adminstore.GetOperation(ctx, f.pool, f.actor, 10, op.ID)
	if err != nil || view.Status != "interrupted" || len(view.Results) != 3 {
		t.Fatalf("changed membership receipt = %+v (%v)", view, err)
	}
	for position, target := range view.Results {
		want := "not_attempted"
		if position == 0 {
			want = "unknown"
		}
		if target.Status != want || target.Code == nil || *target.Code != "membership_changed" || target.Message == nil {
			t.Fatalf("target %d = %+v", position, target)
		}
	}
}

func TestPublicInterruptedViewIsReadOnlyAndKeepsCompletedTargets(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	for i, state := range []string{"missing", "cancelled", "completed", "discarded", "stale", "live"} {
		t.Run(state, func(t *testing.T) {
			op := f.insert(t, i+1, true)
			if ok, err := adminstore.ClaimOperation(ctx, f.pool, op.ID); err != nil || !ok {
				t.Fatalf("claim = %v (%v)", ok, err)
			}
			if err := adminstore.FinishTarget(ctx, f.pool, op.ID, 0, "unchanged", "already_member"); err != nil {
				t.Fatal(err)
			}
			if err := adminstore.StartTarget(ctx, f.pool, op.ID, 1); err != nil {
				t.Fatal(err)
			}
			switch state {
			case "missing":
				f.exec(t, `DELETE FROM river_job WHERE args->>'operation_id' = $1`, op.ID)
			case "stale":
				f.exec(t, `UPDATE river_job SET state = 'running', attempt = 1, attempted_at = now() - interval '31 minutes' WHERE args->>'operation_id' = $1`, op.ID)
			case "live":
				f.exec(t, `UPDATE river_job SET state = 'running', attempt = 1, attempted_at = now() WHERE args->>'operation_id' = $1`, op.ID)
			default:
				f.exec(t, `UPDATE river_job SET state = $2, attempt = 1, attempted_at = now(), finalized_at = now() WHERE args->>'operation_id' = $1`, op.ID, state)
			}
			view, err := adminstore.GetOperation(ctx, f.pool, f.actor, 10, op.ID)
			if err != nil {
				t.Fatal(err)
			}
			if state == "live" {
				if view.Status != "running" || view.CompletedAt != nil || view.Results[1].Status != "running" {
					t.Fatalf("live receipt = %+v", view)
				}
			} else if view.Status != "interrupted" || view.CompletedAt == nil || view.Results[0].Status != "unchanged" ||
				view.Results[1].Status != "unknown" || view.Results[2].Status != "not_attempted" {
				t.Fatalf("interrupted receipt = %+v", view)
			}
			stored, err := adminstore.LoadExecution(ctx, f.pool, op.ID)
			if err != nil || stored.Status != "running" || stored.Results[1].Status != "running" || stored.Results[2].Status != "queued" {
				t.Fatalf("GET changed durable state: %+v (%v)", stored, err)
			}
		})
	}
}

func TestOperationHistoryUsesStrictTimestampAndIDKeyset(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	for i := range 12 {
		f.insert(t, i, false)
	}
	f.exec(t, `UPDATE administration_operations SET created_at = '2026-01-01T00:00:00Z'`)
	first, err := adminstore.ListOperations(ctx, f.pool, f.actor, 10, nil, 10)
	if err != nil || len(first) != 10 || first[0].ID != "receipt-011" {
		t.Fatalf("first page = %+v (%v)", first, err)
	}
	last := first[len(first)-1]
	second, err := adminstore.ListOperations(ctx, f.pool, f.actor, 10, &adminstore.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}, 10)
	if err != nil || len(second) != 2 || second[0].ID != "receipt-001" || second[1].ID != "receipt-000" {
		t.Fatalf("second page = %+v (%v)", second, err)
	}
	if first[0].Status != "interrupted" || first[0].CompletedAt == nil {
		t.Fatalf("history failed to project missing job: %+v", first[0])
	}
}

func TestSafeOutcomeMessagesAndInternalCredentialPrivacy(t *testing.T) {
	if adminstore.OutcomeMessage("role_changed") == "" || adminstore.OutcomeMessage("team_invitation_sent") == "" {
		t.Fatal("missing safe outcome copy")
	}
	if message := adminstore.OutcomeMessage("token-secret private@example.com"); strings.Contains(message, "token-secret") || strings.Contains(message, "@") {
		t.Fatalf("untrusted outcome reflected: %q", message)
	}
	f := newFixture(t)
	op := f.insert(t, 1, false)
	execution, err := adminstore.LoadExecution(context.Background(), f.pool, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(execution)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "session") || strings.Contains(string(data), "SessionHash") || strings.Contains(string(data), "InstallationID") {
		t.Fatalf("internal authorization data serialized: %s", data)
	}
}
