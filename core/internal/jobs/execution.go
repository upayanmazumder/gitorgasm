package jobs

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/upayanmazumder/gitorgasm/core/internal/github"
)

// withSessionLock closes connections whose advisory lock cannot be released.
func withSessionLock(ctx context.Context, pool *pgxpool.Pool, key string, id int64, fn func(*pgxpool.Conn) error) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(`+key+`)`, id).Scan(&locked); err != nil {
		conn.Release()
		return err
	}
	if !locked {
		conn.Release()
		return river.JobSnooze(30 * time.Second)
	}
	err = fn(conn)
	cleanup := context.WithoutCancel(ctx)
	if _, uerr := conn.Exec(cleanup, `SELECT pg_advisory_unlock(`+key+`)`, id); uerr != nil {
		_ = conn.Hijack().Close(cleanup)
		return errors.Join(err, uerr)
	}
	conn.Release()
	return err
}

// retryDecision delays rate limits, stops on permission errors, and otherwise lets River back off.
func retryDecision(err error) error {
	var apiErr *github.APIError
	if !errors.As(err, &apiErr) {
		return err
	}
	switch apiErr.Kind {
	case github.KindRateLimited:
		return river.JobSnooze(apiErr.RetryAfter)
	case github.KindUnauthorized, github.KindForbidden, github.KindInvalid:
		return river.JobCancel(err)
	default:
		return err
	}
}
