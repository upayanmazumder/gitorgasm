package scores

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/store"
)

var ErrPresetLimit = errors.New("too many scoring presets")

// MaxScoringPresets bounds saved presets per installation.
const MaxScoringPresets = 50

type ScoringPreset struct {
	Name  string          `json:"name"`
	Query json.RawMessage `json:"query"`
}

func ListScoringPresets(ctx context.Context, q store.DBTX, installationID, userID int64) ([]ScoringPreset, error) {
	rows, err := q.Query(ctx, `SELECT p.name, p.query FROM scoring_presets p
		JOIN user_installations ui ON ui.installation_id = p.installation_id AND ui.user_id = $2
		WHERE p.installation_id = $1 ORDER BY lower(p.name)`, installationID, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (ScoringPreset, error) {
		var preset ScoringPreset
		err := row.Scan(&preset.Name, &preset.Query)
		return preset, err
	})
}

// GetScoringPresetQuery returns the saved body of a preset, or nil if the caller has no access or it does not exist.
func GetScoringPresetQuery(ctx context.Context, q store.DBTX, installationID, userID int64, name string) (json.RawMessage, error) {
	var query json.RawMessage
	err := q.QueryRow(ctx, `SELECT p.query FROM scoring_presets p
		JOIN user_installations ui ON ui.installation_id = p.installation_id AND ui.user_id = $2
		WHERE p.installation_id = $1 AND p.name = $3`, installationID, userID, name).Scan(&query)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return query, err
}

// UpsertScoringPreset saves a preset if the caller has access and the installation is under the preset limit.
func UpsertScoringPreset(ctx context.Context, q store.DBTX, installationID, userID int64, name string, query json.RawMessage) error {
	tag, err := q.Exec(ctx, `INSERT INTO scoring_presets (installation_id, name, query)
		SELECT ui.installation_id, $3, $4 FROM user_installations ui
		WHERE ui.installation_id = $1 AND ui.user_id = $2
		  AND ((SELECT count(*) FROM scoring_presets WHERE installation_id = $1) < $5
		       OR EXISTS (SELECT 1 FROM scoring_presets WHERE installation_id = $1 AND name = $3))
		ON CONFLICT (installation_id, name) DO UPDATE SET query = EXCLUDED.query, updated_at = now()`,
		installationID, userID, name, query, MaxScoringPresets)
	if err != nil || tag.RowsAffected() > 0 {
		return err
	}
	var granted bool
	if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM user_installations WHERE installation_id = $1 AND user_id = $2)`,
		installationID, userID).Scan(&granted); err != nil {
		return err
	}
	if granted {
		return ErrPresetLimit
	}
	return store.ErrNotFound
}

func DeleteScoringPreset(ctx context.Context, q store.DBTX, installationID, userID int64, name string) (bool, error) {
	tag, err := q.Exec(ctx, `DELETE FROM scoring_presets p USING user_installations ui
		WHERE p.installation_id = $1 AND p.name = $3 AND ui.installation_id = p.installation_id
		AND ui.user_id = $2`, installationID, userID, name)
	return tag.RowsAffected() > 0, err
}
