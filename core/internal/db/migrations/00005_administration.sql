-- +goose Up
-- Receipts deliberately retain only the session hash, never a GitHub credential.
-- river_job_id has no foreign key: application migrations run before River migrations,
-- and River retention must not delete durable administration outcomes.
CREATE TABLE administration_operations (
    id                  TEXT PRIMARY KEY CHECK (id ~ '^[A-Za-z0-9_-]{1,128}$'),
    installation_id     BIGINT NOT NULL REFERENCES installations(github_id),
    account_id          BIGINT NOT NULL REFERENCES accounts(github_id),
    actor_user_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_hash        BYTEA NOT NULL CHECK (octet_length(session_hash) = 32),
    idempotency_key     TEXT NOT NULL CHECK (idempotency_key ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'),
    fingerprint         BYTEA NOT NULL CHECK (octet_length(fingerprint) = 32),
    kind                TEXT NOT NULL CHECK (kind IN ('invite', 'team_membership', 'organization_role')),
    team_slug           TEXT CHECK (team_slug ~ '^[a-z0-9][a-z0-9_-]{0,99}$'),
    role                TEXT CHECK (role IN ('member', 'admin')),
    status              TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'completed', 'interrupted')),
    target_count        INTEGER NOT NULL CHECK (target_count BETWEEN 1 AND 50),
    river_job_id        BIGINT CHECK (river_job_id > 0),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at          TIMESTAMPTZ,
    completed_at        TIMESTAMPTZ,
    UNIQUE (actor_user_id, installation_id, idempotency_key),
    CHECK ((kind = 'team_membership') = (team_slug IS NOT NULL)),
    CHECK ((kind = 'organization_role') = (role IS NOT NULL)),
    CHECK ((status IN ('completed', 'interrupted')) = (completed_at IS NOT NULL)),
    CHECK (status <> 'running' OR started_at IS NOT NULL)
);

CREATE INDEX administration_operations_history_idx
    ON administration_operations (actor_user_id, installation_id, created_at DESC, id DESC);
CREATE INDEX administration_operations_installation_idx ON administration_operations (installation_id);
CREATE INDEX administration_operations_account_idx ON administration_operations (account_id);

CREATE TABLE administration_targets (
    operation_id TEXT NOT NULL REFERENCES administration_operations(id) ON DELETE CASCADE,
    position     INTEGER NOT NULL CHECK (position BETWEEN 0 AND 49),
    login        TEXT NOT NULL CHECK (login ~ '^[A-Za-z0-9][A-Za-z0-9_-]{0,99}$'),
    status       TEXT NOT NULL DEFAULT 'queued' CHECK (status IN (
        'queued', 'running', 'succeeded', 'invited', 'unchanged', 'failed', 'unknown', 'not_attempted'
    )),
    code         TEXT CHECK (code IN (
        'role_changed', 'invitation_sent', 'team_member_added', 'team_invitation_sent',
        'already_member', 'already_invited', 'already_team_member', 'role_unchanged',
        'active_member_required', 'self_demotion_forbidden', 'user_not_found', 'not_a_user',
        'team_not_found', 'github_reauthentication_required', 'not_owner',
        'members_permission_required', 'unsupported_account', 'installation_suspended',
        'github_rate_limited', 'github_unavailable',
        'github_forbidden', 'github_invalid', 'outcome_unknown', 'membership_changed', 'interrupted',
        'session_expired', 'organization_not_found'
    )),
    PRIMARY KEY (operation_id, position),
    CHECK ((status IN ('queued', 'running')) = (code IS NULL)),
    CHECK (status <> 'unknown' OR code IN ('outcome_unknown', 'membership_changed'))
);
CREATE UNIQUE INDEX administration_targets_login_idx ON administration_targets (operation_id, lower(login));

-- +goose Down
DROP TABLE administration_targets;
DROP TABLE administration_operations;
