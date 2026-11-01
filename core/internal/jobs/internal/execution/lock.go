package execution

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

// WithSessionLock closes connections whose advisory lock cannot be released.
func WithSessionLock(ctx context.Context, pool *pgxpool.Pool, key string, id int64, fn func(*pgxpool.Conn) error) error {
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
