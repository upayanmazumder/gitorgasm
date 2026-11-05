package administration

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/store"
)

var errInvalidTransition = errors.New("invalid administration state transition")

// Execution is worker-only metadata. Authorization bindings must never be serialized
// into public receipts, job arguments, audit metadata, or logs.
type Execution struct {
	Operation
	InstallationID int64  `json:"-"`
	AccountID      int64  `json:"-"`
	ActorUserID    int64  `json:"-"`
	SessionHash    []byte `json:"-"`
	Organization   string `json:"-"`
}

// OperationInstallation returns the installation that scopes the worker's advisory lock.
func OperationInstallation(ctx context.Context, q store.DBTX, id string) (int64, error) {
	var installationID int64
	err := q.QueryRow(ctx, `SELECT installation_id FROM administration_operations WHERE id = $1`, id).Scan(&installationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, store.ErrNotFound
	}
	return installationID, err
}

// LoadExecution intentionally does not require a current grant: the worker must be
// able to finalize stopped work after access or a session has been revoked. It does
// not authorize GitHub actions; the same live Authorizer used by HTTP gates those.
func LoadExecution(ctx context.Context, q store.DBTX, id string) (Execution, error) {
	var execution Execution
	err := q.QueryRow(ctx, `SELECT o.installation_id, o.account_id, o.actor_user_id, o.session_hash, a.login
		FROM administration_operations o
		JOIN installations i ON i.github_id = o.installation_id
		JOIN accounts a ON a.github_id = i.account_id
		LEFT JOIN user_installations ui ON ui.installation_id = o.installation_id AND ui.user_id = o.actor_user_id
		WHERE o.id = $1`, id).Scan(&execution.InstallationID, &execution.AccountID, &execution.ActorUserID,
		&execution.SessionHash, &execution.Organization)
	if errors.Is(err, pgx.ErrNoRows) {
		return Execution{}, store.ErrNotFound
	}
	if err != nil {
		return Execution{}, err
	}
	execution.Operation, err = readOperation(ctx, q, rawView, `o.id = $1`, []any{id}, false)
	return execution, err
}

// ClaimOperation never reclaims running work. The worker holds its installation advisory
// lock and interrupts an unexpected running receipt rather than replaying writes.
func ClaimOperation(ctx context.Context, q store.DBTX, id string) (bool, error) {
	tag, err := q.Exec(ctx, `UPDATE administration_operations SET status = 'running', started_at = now()
		WHERE id = $1 AND status = 'queued'`, id)
	return tag.RowsAffected() == 1, err
}

// StartTarget persists the write boundary before the caller sends any GitHub mutation.
func StartTarget(ctx context.Context, q store.DBTX, id string, position int) error {
	tag, err := q.Exec(ctx, `WITH operation AS (
		SELECT id FROM administration_operations WHERE id = $1 AND status = 'running' FOR UPDATE
	)
	UPDATE administration_targets t SET status = 'running'
	FROM operation o WHERE t.operation_id = o.id AND t.position = $2 AND t.status = 'queued'`, id, position)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errInvalidTransition
	}
	return nil
}

// FinishTarget also accepts queued targets, because prechecks can determine a safe
// unchanged/failed outcome without attempting a write. A terminal result is immutable.
func FinishTarget(ctx context.Context, q store.DBTX, id string, position int, status, code string) error {
	if !terminalTargetStatus(status) || OutcomeMessage(code) == "" ||
		(status == "unknown" && code != "outcome_unknown" && code != "membership_changed") {
		return errInvalidTransition
	}
	tag, err := q.Exec(ctx, `WITH operation AS (
		SELECT id FROM administration_operations WHERE id = $1 AND status = 'running' FOR UPDATE
	)
	UPDATE administration_targets t SET status = $3, code = $4
	FROM operation o WHERE t.operation_id = o.id AND t.position = $2 AND t.status IN ('queued', 'running')
		AND ($3 <> 'unknown' OR t.status = 'running')`, id, position, status, code)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errInvalidTransition
	}
	return nil
}

// FinalizeOperation updates the receipt and unfinished targets in one statement,
// serialized with target transitions by the operation row lock. Completed target
// outcomes are retained even when the worker loses permission or is interrupted.
func FinalizeOperation(ctx context.Context, q store.DBTX, id, status, remainingCode string) error {
	if status != "completed" && status != "interrupted" {
		return errInvalidTransition
	}
	if remainingCode == "" {
		remainingCode = "interrupted"
	}
	if OutcomeMessage(remainingCode) == "" {
		return errInvalidTransition
	}
	tag, err := q.Exec(ctx, `WITH locked AS (
		SELECT id, status FROM administration_operations WHERE id = $1 FOR UPDATE
	), eligible AS (
		SELECT id FROM locked WHERE status IN ('queued', 'running')
		AND ($2 = 'interrupted' OR NOT EXISTS (
			SELECT 1 FROM administration_targets WHERE operation_id = $1 AND status IN ('queued', 'running')
		))
	), remaining AS (
		UPDATE administration_targets t SET
			status = CASE WHEN t.status = 'running' THEN 'unknown' ELSE 'not_attempted' END,
			code = CASE WHEN t.status = 'running' THEN 'outcome_unknown' ELSE $3 END
		FROM eligible e WHERE t.operation_id = e.id AND $2 = 'interrupted' AND t.status IN ('queued', 'running')
		RETURNING t.position
	)
	UPDATE administration_operations o SET status = $2, completed_at = now()
	FROM eligible e WHERE o.id = e.id AND (SELECT count(*) FROM remaining) >= 0`, id, status, remainingCode)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 1 {
		return nil
	}
	var existing string
	err = q.QueryRow(ctx, `SELECT status FROM administration_operations WHERE id = $1`, id).Scan(&existing)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ErrNotFound
	}
	if err != nil {
		return err
	}
	if existing == status {
		return nil
	}
	return errInvalidTransition
}

func terminalTargetStatus(status string) bool {
	switch status {
	case "succeeded", "invited", "unchanged", "failed", "unknown", "not_attempted":
		return true
	default:
		return false
	}
}
