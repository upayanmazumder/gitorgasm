-- +goose Up
-- Protect new writes without scanning legacy rows during the upgrade from version 1.
ALTER TABLE users
    ADD CONSTRAINT users_github_id_positive CHECK (github_id > 0) NOT VALID,
    ADD CONSTRAINT users_login_length CHECK (char_length(login) BETWEEN 1 AND 255) NOT VALID,
    ADD CONSTRAINT users_name_length CHECK (char_length(name) <= 1024) NOT VALID,
    ADD CONSTRAINT users_avatar_url_length CHECK (char_length(avatar_url) <= 2048) NOT VALID;

ALTER TABLE sessions
    ADD CONSTRAINT sessions_id_hash_length CHECK (octet_length(id_hash) = 32) NOT VALID,
    ADD CONSTRAINT sessions_key_version_positive CHECK (key_version > 0) NOT VALID,
    ADD CONSTRAINT sessions_token_length CHECK (octet_length(github_token_sealed) <= 4096) NOT VALID,
    ADD CONSTRAINT sessions_refresh_length CHECK (octet_length(github_refresh_sealed) <= 4096) NOT VALID;

ALTER TABLE accounts
    ADD CONSTRAINT accounts_github_id_positive CHECK (github_id > 0) NOT VALID,
    ADD CONSTRAINT accounts_login_length CHECK (char_length(login) BETWEEN 1 AND 255) NOT VALID,
    ADD CONSTRAINT accounts_avatar_url_length CHECK (char_length(avatar_url) <= 2048) NOT VALID;

ALTER TABLE installations
    ADD CONSTRAINT installations_github_id_positive CHECK (github_id > 0) NOT VALID,
    ADD CONSTRAINT installations_permissions_object CHECK (jsonb_typeof(permissions) = 'object' AND pg_column_size(permissions) <= 8192) NOT VALID;

ALTER TABLE repositories
    ADD CONSTRAINT repositories_github_id_positive CHECK (github_id > 0) NOT VALID,
    ADD CONSTRAINT repositories_owner_id_positive CHECK (owner_id > 0) NOT VALID,
    ADD CONSTRAINT repositories_name_length CHECK (char_length(name) BETWEEN 1 AND 255) NOT VALID,
    ADD CONSTRAINT repositories_full_name_length CHECK (char_length(full_name) BETWEEN 3 AND 511) NOT VALID,
    ADD CONSTRAINT repositories_visibility_length CHECK (char_length(visibility) <= 32) NOT VALID,
    ADD CONSTRAINT repositories_branch_length CHECK (char_length(default_branch) <= 255) NOT VALID,
    ADD CONSTRAINT repositories_language_length CHECK (char_length(language) <= 255) NOT VALID,
    ADD CONSTRAINT repositories_stargazers_nonnegative CHECK (stargazers >= 0) NOT VALID;

ALTER TABLE sync_state
    ADD CONSTRAINT sync_state_error_length CHECK (char_length(last_error) <= 2000) NOT VALID,
    ADD CONSTRAINT sync_state_seen_nonnegative CHECK (repositories_seen >= 0) NOT VALID;

ALTER TABLE webhook_deliveries
    ADD CONSTRAINT webhook_delivery_id_length CHECK (char_length(delivery_id) BETWEEN 1 AND 100) NOT VALID,
    ADD CONSTRAINT webhook_event_format CHECK (event ~ '^[a-z_]{1,64}$') NOT VALID,
    ADD CONSTRAINT webhook_action_length CHECK (char_length(action) <= 100) NOT VALID,
    ADD CONSTRAINT webhook_installation_positive CHECK (installation_id > 0) NOT VALID,
    ADD CONSTRAINT webhook_payload_length CHECK (octet_length(payload) <= 26214400) NOT VALID,
    ADD CONSTRAINT webhook_error_length CHECK (char_length(last_error) <= 2000) NOT VALID;

ALTER TABLE audit_events
    ADD CONSTRAINT audit_action_format CHECK (action ~ '^[a-z][a-z_]*(\.[a-z][a-z_]*)+$' AND char_length(action) <= 100) NOT VALID,
    ADD CONSTRAINT audit_target_length CHECK (char_length(target) <= 255) NOT VALID,
    ADD CONSTRAINT audit_request_id_length CHECK (char_length(request_id) <= 64) NOT VALID,
    ADD CONSTRAINT audit_metadata_object CHECK (jsonb_typeof(metadata) = 'object' AND pg_column_size(metadata) <= 8192) NOT VALID;

-- +goose Down
ALTER TABLE audit_events DROP CONSTRAINT audit_action_format, DROP CONSTRAINT audit_target_length, DROP CONSTRAINT audit_request_id_length, DROP CONSTRAINT audit_metadata_object;
ALTER TABLE webhook_deliveries DROP CONSTRAINT webhook_delivery_id_length, DROP CONSTRAINT webhook_event_format, DROP CONSTRAINT webhook_action_length, DROP CONSTRAINT webhook_installation_positive, DROP CONSTRAINT webhook_payload_length, DROP CONSTRAINT webhook_error_length;
ALTER TABLE sync_state DROP CONSTRAINT sync_state_error_length, DROP CONSTRAINT sync_state_seen_nonnegative;
ALTER TABLE repositories DROP CONSTRAINT repositories_github_id_positive, DROP CONSTRAINT repositories_owner_id_positive, DROP CONSTRAINT repositories_name_length, DROP CONSTRAINT repositories_full_name_length, DROP CONSTRAINT repositories_visibility_length, DROP CONSTRAINT repositories_branch_length, DROP CONSTRAINT repositories_language_length, DROP CONSTRAINT repositories_stargazers_nonnegative;
ALTER TABLE installations DROP CONSTRAINT installations_github_id_positive, DROP CONSTRAINT installations_permissions_object;
ALTER TABLE accounts DROP CONSTRAINT accounts_github_id_positive, DROP CONSTRAINT accounts_login_length, DROP CONSTRAINT accounts_avatar_url_length;
ALTER TABLE sessions DROP CONSTRAINT sessions_id_hash_length, DROP CONSTRAINT sessions_key_version_positive, DROP CONSTRAINT sessions_token_length, DROP CONSTRAINT sessions_refresh_length;
ALTER TABLE users DROP CONSTRAINT users_github_id_positive, DROP CONSTRAINT users_login_length, DROP CONSTRAINT users_name_length, DROP CONSTRAINT users_avatar_url_length;
