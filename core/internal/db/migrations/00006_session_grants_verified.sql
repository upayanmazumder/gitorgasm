-- +goose Up
-- When GitHub last confirmed the installation grants through this session's user token.
-- Additive with a default, so the previous release keeps working: its inserts take now().
ALTER TABLE sessions ADD COLUMN grants_verified_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- +goose Down
ALTER TABLE sessions DROP COLUMN grants_verified_at;
