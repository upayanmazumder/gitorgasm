// Package scorer runs the Rust score binary as a local process.
package scorer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/upayanmazumder/gitorgasm/core/internal/store"
)

var (
	// ErrBusy means every scorer slot stayed taken for the whole wait.
	ErrBusy = errors.New("scorer is busy")
	// ErrInvalidQuery means the binary rejected the query or input (exit code 2).
	ErrInvalidQuery = errors.New("score query is invalid")
)

const (
	invalidInputExit = 2
	maxOutputBytes   = 64 << 20
	maxDetailBytes   = 300
)

// Runner bounds concurrent score processes so requests cannot exhaust CPU, memory, or connections.
type Runner struct {
	bin   string
	slots chan struct{}
	wait  time.Duration
}

// New returns a runner for the binary at bin. Callers wait up to wait for a free slot.
func New(bin string, concurrency int, wait time.Duration) *Runner {
	return &Runner{bin: bin, slots: make(chan struct{}, max(concurrency, 1)), wait: wait}
}

func (r *Runner) acquire(ctx context.Context) (func(), error) {
	timer := time.NewTimer(r.wait)
	defer timer.Stop()
	select {
	case r.slots <- struct{}{}:
		return func() { <-r.slots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return nil, ErrBusy
	}
}

// Validate checks a query without reading events.
func (r *Runner) Validate(ctx context.Context, query json.RawMessage) error {
	release, err := r.acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	cmd := exec.CommandContext(ctx, r.bin, "--validate-query") //nolint:gosec // binary path comes from validated config
	cmd.Stdin = bytes.NewReader(query)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	return exitError("score query validation", cmd.Run(), &stderr)
}

// Calculate streams stored events to the binary inside a read-only transaction whose
// statement timeout covers the whole stream.
func (r *Runner) Calculate(ctx context.Context, db pgx.Tx, installationID, userID int64, organization string, query json.RawMessage) (json.RawMessage, error) {
	release, err := r.acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	cmd := exec.CommandContext(ctx, r.bin) //nolint:gosec // binary path comes from validated config
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout := &limitedBuffer{limit: maxOutputBytes}
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = stdout, &stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	buffered := bufio.NewWriterSize(stdin, 64<<10)
	writeErr := writeInput(ctx, buffered, db, installationID, userID, organization, query)
	if writeErr == nil {
		writeErr = buffered.Flush()
	}
	if err := stdin.Close(); writeErr == nil {
		writeErr = err
	}
	waitErr := cmd.Wait()
	// A process that exits early breaks the pipe; then its own error explains why.
	if writeErr != nil && !errors.Is(writeErr, syscall.EPIPE) {
		return nil, writeErr
	}
	if err := exitError("score calculation", waitErr, &stderr); err != nil {
		return nil, err
	}
	if writeErr != nil {
		return nil, writeErr
	}
	if stdout.exceeded {
		return nil, fmt.Errorf("score calculation: output exceeds %d bytes", maxOutputBytes)
	}
	if !json.Valid(stdout.Bytes()) {
		return nil, errors.New("score calculation: invalid JSON output")
	}
	return json.RawMessage(stdout.Bytes()), nil
}

// exitError keeps only the first line of stderr, bounded, so logs never carry input payloads.
func exitError(op string, err error, stderr *bytes.Buffer) error {
	if err == nil {
		return nil
	}
	detail, _, _ := strings.Cut(stderr.String(), "\n")
	if len(detail) > maxDetailBytes {
		detail = detail[:maxDetailBytes]
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == invalidInputExit {
		return fmt.Errorf("%s: %w: %s", op, ErrInvalidQuery, detail)
	}
	return fmt.Errorf("%s: %w: %s", op, err, detail)
}

type limitedBuffer struct {
	bytes.Buffer
	limit    int
	exceeded bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		b.exceeded = true
		return 0, errors.New("output limit exceeded")
	}
	return b.Buffer.Write(p)
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
