// Package scorer runs the Rust score binary as a local process.
package scorer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
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
// One user and one installation can each hold at most max(concurrency-1, 1) slots, so a single
// tenant cannot starve the others of the last free slot.
type Runner struct {
	bin           string
	slots         chan struct{}
	wait          time.Duration
	users         *tenantLimiter
	installations *tenantLimiter
}

// New returns a runner for the binary at bin. Callers wait up to wait for a free slot.
func New(bin string, concurrency int, wait time.Duration) *Runner {
	concurrency = max(concurrency, 1)
	perTenant := max(concurrency-1, 1)
	return &Runner{
		bin: bin, slots: make(chan struct{}, concurrency), wait: wait,
		users: newTenantLimiter(perTenant), installations: newTenantLimiter(perTenant),
	}
}

// acquire takes the caller's tenant slots, then a process slot, all within one wait budget.
// Tenant slots come first so a waiting caller never holds a process slot another tenant needs.
// A zero userID (worker jobs) has no per-user limit.
func (r *Runner) acquire(ctx context.Context, installationID, userID int64) (func(), error) {
	timer := time.NewTimer(r.wait)
	defer timer.Stop()
	var releases []func()
	releaseAll := func() {
		for i := len(releases) - 1; i >= 0; i-- {
			releases[i]()
		}
	}
	if userID != 0 {
		release, err := r.users.acquire(ctx, timer.C, tenantKey{kindUser, userID})
		if err != nil {
			return nil, err
		}
		releases = append(releases, release)
	}
	if installationID != 0 {
		release, err := r.installations.acquire(ctx, timer.C, tenantKey{kindInstallation, installationID})
		if err != nil {
			releaseAll()
			return nil, err
		}
		releases = append(releases, release)
	}
	select {
	case r.slots <- struct{}{}:
		releases = append(releases, func() { <-r.slots })
		return releaseAll, nil
	case <-ctx.Done():
		releaseAll()
		return nil, ctx.Err()
	case <-timer.C:
		releaseAll()
		return nil, ErrBusy
	}
}

// Validate checks a query without reading events. The caller's installation and user share the tenant limits.
func (r *Runner) Validate(ctx context.Context, installationID, userID int64, query json.RawMessage) error {
	release, err := r.acquire(ctx, installationID, userID)
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

// Calculate streams events within a read-only transaction whose timeout covers the whole stream.
func (r *Runner) Calculate(ctx context.Context, db pgx.Tx, installationID, userID int64, organization string, query json.RawMessage) (json.RawMessage, error) {
	release, err := r.acquire(ctx, installationID, userID)
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
