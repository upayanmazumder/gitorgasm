-- +goose Up
CREATE TABLE contribution_runs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    installation_id BIGINT NOT NULL REFERENCES installations(github_id) ON DELETE CASCADE,
    window_start TIMESTAMPTZ NOT NULL,
    window_end TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    CHECK (window_start < window_end)
);
CREATE INDEX contribution_runs_installation_idx ON contribution_runs (installation_id, id DESC);

CREATE TABLE staged_contributions (
    run_id BIGINT NOT NULL REFERENCES contribution_runs(id) ON DELETE CASCADE,
    repository_id BIGINT NOT NULL REFERENCES repositories(github_id) ON DELETE CASCADE,
    event_id TEXT NOT NULL,
    user_id BIGINT NOT NULL,
    user_login TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('commit', 'pr_open', 'pr_merge', 'review', 'issue_open', 'issue_close')),
    occurred_at TIMESTAMPTZ NOT NULL,
    is_bot BOOLEAN NOT NULL,
    PRIMARY KEY (run_id, repository_id, event_id)
);

CREATE TABLE contribution_events (
    installation_id BIGINT NOT NULL REFERENCES installations(github_id) ON DELETE CASCADE,
    repository_id BIGINT NOT NULL REFERENCES repositories(github_id) ON DELETE CASCADE,
    event_id TEXT NOT NULL,
    user_id BIGINT NOT NULL,
    user_login TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('commit', 'pr_open', 'pr_merge', 'review', 'issue_open', 'issue_close')),
    occurred_at TIMESTAMPTZ NOT NULL,
    is_bot BOOLEAN NOT NULL,
    PRIMARY KEY (installation_id, repository_id, event_id)
);
CREATE INDEX contribution_events_period_idx ON contribution_events (installation_id, occurred_at, user_id);

CREATE TABLE scoring_state (
    installation_id BIGINT PRIMARY KEY REFERENCES installations(github_id) ON DELETE CASCADE,
    last_run_id BIGINT REFERENCES contribution_runs(id) ON DELETE SET NULL,
    generation BIGINT NOT NULL DEFAULT 0 CHECK (generation >= 0),
    scored_generation BIGINT NOT NULL DEFAULT 0 CHECK (scored_generation >= 0),
    last_ingested_at TIMESTAMPTZ,
    last_scored_at TIMESTAMPTZ,
    CHECK (scored_generation <= generation)
);

CREATE TABLE scoring_presets (
    installation_id BIGINT NOT NULL REFERENCES installations(github_id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    query JSONB NOT NULL CHECK (jsonb_typeof(query) = 'object'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (installation_id, name)
);

CREATE TABLE score_snapshots (
    installation_id BIGINT NOT NULL REFERENCES installations(github_id) ON DELETE CASCADE,
    generation BIGINT NOT NULL,
    query_key TEXT NOT NULL,
    result JSONB NOT NULL CHECK (jsonb_typeof(result) = 'object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (installation_id, generation, query_key)
);

-- +goose Down
DROP TABLE score_snapshots;
DROP TABLE scoring_presets;
DROP TABLE scoring_state;
DROP TABLE contribution_events;
DROP TABLE staged_contributions;
DROP TABLE contribution_runs;
