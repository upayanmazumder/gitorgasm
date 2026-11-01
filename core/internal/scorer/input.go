package scorer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/store"
	"github.com/upayanmazumder/gitorgasm/core/internal/store/contributions"
)

func writeInput(ctx context.Context, w io.Writer, q store.DBTX, installationID, userID int64, organization string, query json.RawMessage) error {
	rows, err := contributions.ContributionRows(ctx, q, installationID, userID)
	if err != nil {
		return err
	}
	defer rows.Close()
	name, err := json.Marshal(organization)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, `{"organization":%s,"memberships":[],"query":%s,"signals":[`, name, query); err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	first := true
	for rows.Next() {
		var id int64
		var login, kind, repo string
		var at time.Time
		var bot bool
		if err := rows.Scan(&id, &login, &kind, &repo, &at, &bot); err != nil {
			return err
		}
		if !first {
			if _, err := io.WriteString(w, ","); err != nil {
				return err
			}
		}
		first = false
		signal := struct {
			UserID         int64     `json:"userId"`
			UserLogin      string    `json:"userLogin"`
			Kind           string    `json:"type"`
			Repo           string    `json:"repo"`
			EventTimestamp time.Time `json:"eventTimestamp"`
			Metadata       struct {
				IsBot bool `json:"isBot"`
			} `json:"metadata"`
		}{UserID: id, UserLogin: login, Kind: kind, Repo: repo, EventTimestamp: at}
		signal.Metadata.IsBot = bot
		if err := enc.Encode(signal); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = io.WriteString(w, `]}`)
	return err
}

// ReadOnly runs fn in a read-only transaction with its own statement timeout.
func ReadOnly(ctx context.Context, db interface {
	BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
}, statementTimeout time.Duration, fn func(pgx.Tx) error) error {
	return pgx.BeginTxFunc(ctx, db, pgx.TxOptions{AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL statement_timeout = %d", statementTimeout.Milliseconds())); err != nil {
			return err
		}
		return fn(tx)
	})
}
