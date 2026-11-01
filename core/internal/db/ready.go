package db

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// startupWait bounds how long a starting process waits for PostgreSQL to accept connections.
const startupWait = 30 * time.Second

// transientStartupError reports failures that clear once PostgreSQL finishes starting.
// Authentication, DSN, and TLS errors are not transient and must fail fast.
func transientStartupError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "57P03"
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

// retryUntilReady calls ping until it succeeds, fails with a non-retryable error, or ctx ends.
// It returns the last ping error.
func retryUntilReady(
	ctx context.Context,
	ping func(context.Context) error,
	retryable func(error) bool,
	every time.Duration,
) error {
	for {
		err := ping(ctx)
		if err == nil || !retryable(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return err
		case <-time.After(every):
		}
	}
}
