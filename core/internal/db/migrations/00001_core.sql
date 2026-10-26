-- +goose Up
-- Core identity, tenancy, and GitHub mirror tables.
-- GitHub-owned data (accounts, installations, repositories) is a mirror. GitHub stays the source of truth.

CREATE TABLE users (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    github_id   BIGINT NOT NULL UNIQUE,
    login       TEXT   NOT NULL,
    name        TEXT,
    avatar_url  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Session ids are random 256-bit values. Only their SHA-256 hash is stored.
-- GitHub user tokens are sealed with AES-256-GCM. key_version supports key rotation.
CREATE TABLE sessions (
    id_hash                 BYTEA PRIMARY KEY,
    user_id                 BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_version             SMALLINT NOT NULL,
    github_token_sealed     BYTEA NOT NULL,
    github_refresh_sealed   BYTEA,
    github_token_expires_at TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at              TIMESTAMPTZ NOT NULL
);
CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

-- GitHub accounts (organizations or users) that own installations.
CREATE TABLE accounts (
    github_id   BIGINT PRIMARY KEY,
    login       TEXT NOT NULL,
    type        TEXT NOT NULL CHECK (type IN ('Organization', 'User', 'Enterprise')),
    avatar_url  TEXT,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX accounts_login_lower_idx ON accounts (lower(login));

CREATE TABLE installations (
    github_id            BIGINT PRIMARY KEY,
    account_id           BIGINT NOT NULL REFERENCES accounts(github_id),
    repository_selection TEXT NOT NULL CHECK (repository_selection IN ('all', 'selected')),
    permissions          JSONB NOT NULL DEFAULT '{}'::jsonb,
    suspended_at         TIMESTAMPTZ,
    deleted_at           TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX installations_account_id_idx ON installations (account_id);

-- Tenant authorization edge. A row exists only when GitHub reported, through
-- GET /user/installations with the user's token, that the user can access the installation.
CREATE TABLE user_installations (
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    installation_id BIGINT NOT NULL REFERENCES installations(github_id) ON DELETE CASCADE,
    verified_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, installation_id)
);
CREATE INDEX user_installations_installation_idx ON user_installations (installation_id);

CREATE TABLE repositories (
    github_id       BIGINT PRIMARY KEY,
    installation_id BIGINT NOT NULL REFERENCES installations(github_id) ON DELETE CASCADE,
    owner_id        BIGINT NOT NULL,
    name            TEXT NOT NULL,
    full_name       TEXT NOT NULL,
    description     TEXT,
    private         BOOLEAN NOT NULL,
    fork            BOOLEAN NOT NULL,
    archived        BOOLEAN NOT NULL,
    visibility      TEXT,
    default_branch  TEXT,
    language        TEXT,
    stargazers      INTEGER NOT NULL DEFAULT 0,
    pushed_at       TIMESTAMPTZ,
    github_updated_at TIMESTAMPTZ,
    synced_at       TIMESTAMPTZ NOT NULL,
    deleted_at      TIMESTAMPTZ
);
-- Keyset pagination for the repository list: (installation, lower(name), id).
CREATE INDEX repositories_listing_idx ON repositories (installation_id, lower(name), github_id) WHERE deleted_at IS NULL;

-- Per-installation synchronization checkpoint.
CREATE TABLE sync_state (
    installation_id     BIGINT PRIMARY KEY REFERENCES installations(github_id) ON DELETE CASCADE,
    last_started_at     TIMESTAMPTZ,
    last_succeeded_at   TIMESTAMPTZ,
    last_error          TEXT,
    repositories_seen   INTEGER NOT NULL DEFAULT 0
);

-- Durable webhook inbox. The delivery GUID makes redelivery idempotent.
CREATE TABLE webhook_deliveries (
    delivery_id     TEXT PRIMARY KEY,
    event           TEXT NOT NULL,
    action          TEXT,
    installation_id BIGINT,
    -- Raw signed body. BYTEA, not JSONB, because JSONB rejects \u0000 escapes that GitHub may send.
    payload         BYTEA NOT NULL,
    received_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at    TIMESTAMPTZ,
    last_error      TEXT
);
CREATE INDEX webhook_deliveries_unprocessed_idx ON webhook_deliveries (received_at) WHERE processed_at IS NULL;

-- Append-only audit trail of security-relevant actions.
CREATE TABLE audit_events (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    occurred_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    action        TEXT NOT NULL,
    target        TEXT,
    request_id    TEXT,
    metadata      JSONB NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX audit_events_actor_idx ON audit_events (actor_user_id, occurred_at DESC);

-- +goose Down
DROP TABLE audit_events;
DROP TABLE webhook_deliveries;
DROP TABLE sync_state;
DROP TABLE repositories;
DROP TABLE user_installations;
DROP TABLE installations;
DROP TABLE accounts;
DROP TABLE sessions;
DROP TABLE users;
