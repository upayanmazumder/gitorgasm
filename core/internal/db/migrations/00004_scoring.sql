-- +goose Up
-- One ingestion pass per installation. Incomplete runs resume from per-repository cursors.
CREATE TABLE contribution_runs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    installation_id BIGINT NOT NULL REFERENCES installations(github_id) ON DELETE CASCADE,
    window_end TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);
CREATE INDEX contribution_runs_installation_idx ON contribution_runs (installation_id, id DESC);

-- Progress of one repository inside a run. Phase 'done' or 'skipped' ends it.
CREATE TABLE contribution_run_repositories (
    run_id BIGINT NOT NULL REFERENCES contribution_runs(id) ON DELETE CASCADE,
    repository_id BIGINT NOT NULL REFERENCES repositories(github_id) ON DELETE CASCADE,
    commits_since TIMESTAMPTZ NOT NULL,
    activity_since TIMESTAMPTZ NOT NULL,
    phase TEXT NOT NULL CHECK (phase IN ('commits', 'pulls', 'issues', 'done', 'skipped')),
    next_url TEXT NOT NULL DEFAULT '' CHECK (char_length(next_url) <= 2000),
    PRIMARY KEY (run_id, repository_id)
);
CREATE INDEX contribution_run_repositories_repository_idx ON contribution_run_repositories (repository_id);

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
CREATE INDEX staged_contributions_repository_idx ON staged_contributions (repository_id);

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
CREATE INDEX contribution_events_repository_idx ON contribution_events (repository_id, kind, occurred_at);

-- Events of a repository are complete up to ingested_until.
CREATE TABLE repository_ingest_state (
    repository_id BIGINT PRIMARY KEY REFERENCES repositories(github_id) ON DELETE CASCADE,
    ingested_until TIMESTAMPTZ NOT NULL
);

CREATE TABLE scoring_state (
    installation_id BIGINT PRIMARY KEY REFERENCES installations(github_id) ON DELETE CASCADE,
    generation BIGINT NOT NULL DEFAULT 0 CHECK (generation >= 0),
    scored_generation BIGINT NOT NULL DEFAULT 0 CHECK (scored_generation >= 0),
    last_ingested_at TIMESTAMPTZ,
    last_scored_at TIMESTAMPTZ,
    last_failed_at TIMESTAMPTZ,
    last_error TEXT CHECK (char_length(last_error) <= 2000),
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
DROP TABLE repository_ingest_state;
DROP TABLE contribution_events;
DROP TABLE staged_contributions;
DROP TABLE contribution_run_repositories;
DROP TABLE contribution_runs;
