// Package administration stores durable, actor-scoped organization administration receipts.
package administration

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/store"
)

var ErrIdempotencyConflict = errors.New("idempotency key already used for another request")

// NewOperation contains intent and a non-secret session binding, never a user token.
type NewOperation struct {
	ID             string
	InstallationID int64
	AccountID      int64
	ActorUserID    int64
	SessionHash    []byte
	IdempotencyKey string
	Fingerprint    []byte
	Kind           string
	TeamSlug       string
	Role           string
	Logins         []string
}

type OperationSummary struct {
	ID          string     `json:"id"`
	Kind        string     `json:"kind"`
	Status      string     `json:"status"`
	TeamSlug    *string    `json:"team_slug"`
	Role        *string    `json:"role"`
	TargetCount int        `json:"target_count"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

type TargetResult struct {
	Login   string  `json:"login"`
	Status  string  `json:"status"`
	Code    *string `json:"code"`
	Message *string `json:"message"`
}

type Operation struct {
	OperationSummary
	Results []TargetResult `json:"results"`
}

type Cursor struct {
	CreatedAt time.Time
	ID        string
}

const publicScope = `
	JOIN installations i ON i.github_id = o.installation_id
		AND i.account_id = o.account_id AND i.deleted_at IS NULL
	JOIN user_installations ui ON ui.installation_id = o.installation_id
		AND ui.user_id = o.actor_user_id`

// A missing, terminal, or abandoned job can never leave a publicly active receipt.
// This is a SELECT-only projection: a read must not race the worker by mutating progress.
const interruptedView = `
	LEFT JOIN river_job j ON j.id = o.river_job_id
	CROSS JOIN LATERAL (
		SELECT o.status IN ('queued', 'running') AND (
			j.id IS NULL OR j.state IN ('cancelled', 'completed', 'discarded') OR
			(j.state = 'running' AND
				COALESCE(j.attempted_at, o.started_at, o.created_at) < now() - interval '30 minutes')
		) AS interrupted
	) stopped
	CROSS JOIN LATERAL (
		SELECT CASE WHEN stopped.interrupted THEN 'interrupted' ELSE o.status END AS status,
			CASE WHEN stopped.interrupted THEN
				COALESCE(j.finalized_at,
					CASE WHEN j.state = 'running' THEN COALESCE(j.attempted_at, o.started_at, o.created_at) + interval '30 minutes' END,
					o.started_at, o.created_at)
			ELSE o.completed_at END AS completed_at
	) receipt`

const rawView = `
	CROSS JOIN LATERAL (SELECT false AS interrupted) stopped
	CROSS JOIN LATERAL (SELECT o.status, o.completed_at) receipt`

const summaryColumns = `o.id, o.kind, receipt.status, o.team_slug, o.role,
	o.target_count, o.created_at, receipt.completed_at`

// InsertOperation participates in the caller's enqueue transaction. The unique index
// serializes concurrent same-key submissions; the second statement sees the committed
// winner at PostgreSQL's normal READ COMMITTED isolation level.
// The returned receipt is durable state, without the interrupted projection: a newly
// created operation does not have its transactional River job attached yet.
func InsertOperation(ctx context.Context, tx pgx.Tx, in NewOperation) (Operation, bool, error) {
	key := strings.ToLower(in.IdempotencyKey)
	var id string
	err := tx.QueryRow(ctx, `
		INSERT INTO administration_operations
			(id, installation_id, account_id, actor_user_id, session_hash, idempotency_key,
			 fingerprint, kind, team_slug, role, target_count)
		SELECT $1, i.github_id, i.account_id, ui.user_id, $5, $6, $7, $8,
			CASE WHEN $8 = 'team_membership' THEN $9::text ELSE NULL END,
			CASE WHEN $8 = 'organization_role' THEN $10::text ELSE NULL END, $11
		FROM installations i
		JOIN user_installations ui ON ui.installation_id = i.github_id AND ui.user_id = $4
		WHERE i.github_id = $2 AND i.account_id = $3 AND i.deleted_at IS NULL
		ON CONFLICT (actor_user_id, installation_id, idempotency_key) DO NOTHING
		RETURNING id`, in.ID, in.InstallationID, in.AccountID, in.ActorUserID,
		in.SessionHash, key, in.Fingerprint, in.Kind, in.TeamSlug, in.Role, len(in.Logins)).Scan(&id)
	created := err == nil
	if errors.Is(err, pgx.ErrNoRows) {
		var fingerprint []byte
		err = tx.QueryRow(ctx, `SELECT o.id, o.fingerprint FROM administration_operations o`+publicScope+`
			WHERE o.actor_user_id = $1 AND o.installation_id = $2 AND o.idempotency_key = $3
			AND o.account_id = $4 FOR UPDATE OF o`, in.ActorUserID, in.InstallationID, key, in.AccountID).
			Scan(&id, &fingerprint)
		if errors.Is(err, pgx.ErrNoRows) {
			return Operation{}, false, store.ErrNotFound
		}
		if err != nil {
			return Operation{}, false, err
		}
		if !bytes.Equal(fingerprint, in.Fingerprint) {
			return Operation{}, false, ErrIdempotencyConflict
		}
	} else if err != nil {
		return Operation{}, false, err
	}
	if created {
		if _, err := tx.Exec(ctx, `INSERT INTO administration_targets (operation_id, position, login)
			SELECT $1, (ordinality - 1)::integer, login
			FROM unnest($2::text[]) WITH ORDINALITY AS targets(login, ordinality)`, id, in.Logins); err != nil {
			return Operation{}, false, err
		}
	}
	op, err := readOperation(ctx, tx, rawView, `o.id = $1`, []any{id}, false)
	return op, created, err
}

// SetJobID is called after InsertTx and before the submission transaction commits.
func SetJobID(ctx context.Context, q store.DBTX, id string, jobID int64) error {
	tag, err := q.Exec(ctx, `UPDATE administration_operations SET river_job_id = $2
		WHERE id = $1 AND status = 'queued' AND (river_job_id IS NULL OR river_job_id = $2)`, id, jobID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errInvalidTransition
	}
	return nil
}

// HasIdempotencyKey avoids upstream resource revalidation for an existing receipt.
// InsertOperation still owns atomic fingerprint comparison and enqueue uniqueness.
func HasIdempotencyKey(ctx context.Context, q store.DBTX, actorUserID, installationID int64, key string) (bool, error) {
	var exists bool
	err := q.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM administration_operations o`+publicScope+`
		WHERE o.actor_user_id = $1 AND o.installation_id = $2 AND o.idempotency_key = $3
	)`, actorUserID, installationID, strings.ToLower(key)).Scan(&exists)
	return exists, err
}

func GetOperation(ctx context.Context, q store.DBTX, actorUserID, installationID int64, id string) (Operation, error) {
	return readOperation(ctx, q, interruptedView, `o.actor_user_id = $1 AND o.installation_id = $2 AND o.id = $3`,
		[]any{actorUserID, installationID, id}, true)
}

// readOperation uses one statement for the summary and targets, so progress cannot
// produce a completed summary paired with an older, unfinished result snapshot.
// view and predicate are private SQL constants, never caller-supplied strings.
func readOperation(ctx context.Context, q store.DBTX, view, predicate string, args []any, scoped bool) (Operation, error) {
	scope := ""
	if scoped {
		scope = publicScope
	}
	rows, err := q.Query(ctx, `SELECT `+summaryColumns+`, t.login,
		CASE WHEN stopped.interrupted AND t.status = 'running' THEN 'unknown'
			WHEN stopped.interrupted AND t.status = 'queued' THEN 'not_attempted' ELSE t.status END,
		CASE WHEN stopped.interrupted AND t.status = 'running' THEN 'outcome_unknown'
			WHEN stopped.interrupted AND t.status = 'queued' THEN 'interrupted' ELSE t.code END
		FROM administration_operations o`+scope+view+`
		JOIN administration_targets t ON t.operation_id = o.id
		WHERE `+predicate+` ORDER BY t.position`, args...)
	if err != nil {
		return Operation{}, err
	}
	defer rows.Close()
	op := Operation{Results: make([]TargetResult, 0, 50)}
	for rows.Next() {
		var target TargetResult
		if err := rows.Scan(&op.ID, &op.Kind, &op.Status, &op.TeamSlug, &op.Role,
			&op.TargetCount, &op.CreatedAt, &op.CompletedAt, &target.Login, &target.Status, &target.Code); err != nil {
			return Operation{}, err
		}
		if target.Code != nil {
			message := OutcomeMessage(*target.Code)
			target.Message = &message
		}
		op.Results = append(op.Results, target)
	}
	if err := rows.Err(); err != nil {
		return Operation{}, err
	}
	if len(op.Results) == 0 {
		return Operation{}, store.ErrNotFound
	}
	return op, nil
}

// ListOperations returns bounded summaries with strict descending timestamp/ID keysets.
func ListOperations(ctx context.Context, q store.DBTX, actorUserID, installationID int64, cursor *Cursor, limit int) ([]OperationSummary, error) {
	if limit < 1 || limit > 51 {
		return nil, errors.New("invalid administration history limit")
	}
	var at *time.Time
	var id string
	if cursor != nil {
		at, id = &cursor.CreatedAt, cursor.ID
	}
	rows, err := q.Query(ctx, `SELECT `+summaryColumns+` FROM administration_operations o`+publicScope+interruptedView+`
		WHERE o.actor_user_id = $1 AND o.installation_id = $2
		AND ($3::timestamptz IS NULL OR (o.created_at, o.id) < ($3::timestamptz, $4::text))
		ORDER BY o.created_at DESC, o.id DESC LIMIT $5`, actorUserID, installationID, at, id, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	operations := make([]OperationSummary, 0, limit)
	for rows.Next() {
		var op OperationSummary
		if err := rows.Scan(&op.ID, &op.Kind, &op.Status, &op.TeamSlug, &op.Role,
			&op.TargetCount, &op.CreatedAt, &op.CompletedAt); err != nil {
			return nil, err
		}
		operations = append(operations, op)
	}
	return operations, rows.Err()
}
