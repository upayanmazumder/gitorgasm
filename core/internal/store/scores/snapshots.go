package scores

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/store"
)

var ErrStaleGeneration = errors.New("scoring generation changed")

// MaxQuerySnapshots bounds cached custom queries per installation and generation.
const MaxQuerySnapshots = 200

type ScoringState struct {
	Generation       int64      `json:"generation"`
	ScoredGeneration int64      `json:"scored_generation"`
	LastIngestedAt   *time.Time `json:"last_ingested_at"`
	LastScoredAt     *time.Time `json:"last_scored_at"`
	IngestionFailed  bool       `json:"ingestion_failed"`
}

func GetScoringState(ctx context.Context, q store.DBTX, installationID, userID int64) (ScoringState, error) {
	var s ScoringState
	err := q.QueryRow(ctx, `SELECT COALESCE(state.generation, 0), COALESCE(state.scored_generation, 0),
		state.last_ingested_at, state.last_scored_at, state.last_failed_at IS NOT NULL
		FROM installations i
		LEFT JOIN scoring_state state ON state.installation_id = i.github_id
		LEFT JOIN user_installations ui ON ui.installation_id = i.github_id AND ui.user_id = $2
		WHERE i.github_id = $1 AND i.deleted_at IS NULL AND ($2 = 0 OR ui.user_id IS NOT NULL)`,
		installationID, userID).
		Scan(&s.Generation, &s.ScoredGeneration, &s.LastIngestedAt, &s.LastScoredAt, &s.IngestionFailed)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, store.ErrNotFound
	}
	return s, err
}

func StoreSnapshot(ctx context.Context, tx pgx.Tx, installationID, generation, userID int64, queryKey string, result json.RawMessage, markScored bool) error {
	var current int64
	err := tx.QueryRow(ctx, `SELECT state.generation FROM scoring_state state
		LEFT JOIN user_installations ui ON ui.installation_id = state.installation_id AND ui.user_id = $2
		WHERE state.installation_id = $1 AND ($2 = 0 OR ui.user_id IS NOT NULL) FOR UPDATE OF state`,
		installationID, userID).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ErrNotFound
	}
	if err != nil {
		return err
	}
	if current != generation {
		return ErrStaleGeneration
	}
	if _, err := tx.Exec(ctx, `INSERT INTO score_snapshots (installation_id, generation, query_key, result)
		VALUES ($1, $2, $3, $4) ON CONFLICT (installation_id, generation, query_key)
		DO UPDATE SET result = EXCLUDED.result, created_at = now()`, installationID, generation, queryKey, result); err != nil {
		return err
	}
	if markScored {
		if _, err := tx.Exec(ctx, `UPDATE scoring_state SET scored_generation = $2, last_scored_at = now()
			WHERE installation_id = $1`, installationID, generation); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM score_snapshots WHERE installation_id = $1 AND generation < $2`, installationID, generation)
		return err
	}
	_, err = tx.Exec(ctx, `DELETE FROM score_snapshots WHERE installation_id = $1 AND generation = $2
		AND query_key <> 'default' AND query_key NOT IN (
		  SELECT query_key FROM score_snapshots WHERE installation_id = $1 AND generation = $2 AND query_key <> 'default'
		   ORDER BY created_at DESC LIMIT $3)`, installationID, generation, MaxQuerySnapshots)
	return err
}

func LoadSnapshot(ctx context.Context, q store.DBTX, installationID, generation, userID int64, queryKey string) (json.RawMessage, error) {
	var data json.RawMessage
	err := q.QueryRow(ctx, `SELECT ss.result FROM score_snapshots ss
		LEFT JOIN user_installations ui ON ui.installation_id = ss.installation_id AND ui.user_id = $4
		WHERE ss.installation_id = $1 AND ss.generation = $2 AND ss.query_key = $3
		AND ($4 = 0 OR ui.user_id IS NOT NULL)`, installationID, generation, queryKey, userID).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return data, err
}
