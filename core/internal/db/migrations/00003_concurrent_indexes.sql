-- +goose NO TRANSACTION
-- +goose Up
-- Keep lookups indexed while allowing GitHub to reuse renamed account logins.
CREATE INDEX CONCURRENTLY IF NOT EXISTS accounts_login_recent_idx ON accounts (lower(login), updated_at DESC);
DROP INDEX CONCURRENTLY IF EXISTS accounts_login_lower_idx;
CREATE INDEX CONCURRENTLY IF NOT EXISTS webhook_deliveries_received_at_idx ON webhook_deliveries (received_at);

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS webhook_deliveries_received_at_idx;
-- Keep the non-unique login index; restoring uniqueness could reject valid accounts.
