package scorer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/upayanmazumder/gitorgasm/core/internal/store"
)

func Binary() string {
	if path := os.Getenv("SCORING_BIN"); path != "" {
		return path
	}
	return "/gitorgasm-score"
}

func Validate(ctx context.Context, query json.RawMessage) error {
	cmd := exec.CommandContext(ctx, Binary(), "--validate-query") //nolint:gosec // operator-configured binary path
	cmd.Stdin = bytes.NewReader(query)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("score query validation: %w: %s", err, out)
	}
	return nil
}

// Calculate streams the stored events to the Rust scorer without retaining all events in Go.
func Calculate(ctx context.Context, q store.DBTX, installationID, userID int64, organization string, query json.RawMessage) (json.RawMessage, error) {
	cmd := exec.CommandContext(ctx, Binary()) //nolint:gosec // operator-configured binary path
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	writeErr := writeInput(ctx, stdin, q, installationID, userID, organization, query)
	if err := stdin.Close(); writeErr == nil {
		writeErr = err
	}
	if writeErr != nil {
		_ = cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	if writeErr != nil {
		return nil, writeErr
	}
	if waitErr != nil {
		return nil, fmt.Errorf("score calculation: %w: %s", waitErr, stderr.String())
	}
	if !json.Valid(stdout.Bytes()) {
		return nil, fmt.Errorf("scorer returned invalid JSON")
	}
	return json.RawMessage(stdout.Bytes()), nil
}

func writeInput(ctx context.Context, w io.Writer, q store.DBTX, installationID, userID int64, organization string, query json.RawMessage) error {
	rows, err := store.ContributionRows(ctx, q, installationID, userID)
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
