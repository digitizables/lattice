-- +goose Up
CREATE INDEX agents_last_seen_id_idx ON agents (last_seen DESC, id DESC);
CREATE INDEX agents_hostname_idx ON agents (hostname);
CREATE INDEX agents_username_idx ON agents (username);
CREATE INDEX agents_os_idx ON agents (os);

-- +goose Down
DROP INDEX IF EXISTS agents_os_idx;
DROP INDEX IF EXISTS agents_username_idx;
DROP INDEX IF EXISTS agents_hostname_idx;
DROP INDEX IF EXISTS agents_last_seen_id_idx;
