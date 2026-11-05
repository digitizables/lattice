-- +goose Up
ALTER TABLE agents ADD COLUMN IF NOT EXISTS note TEXT NOT NULL DEFAULT '';
ALTER TABLE agents ADD COLUMN IF NOT EXISTS jitter_percent INTEGER NOT NULL DEFAULT 0;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS features TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE agents ADD COLUMN IF NOT EXISTS ops TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE agents ADD COLUMN IF NOT EXISTS desired_sleep_seconds BIGINT;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS desired_jitter_percent INTEGER;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS has_desired_sleep BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS loot (
    id            TEXT PRIMARY KEY,
    agent_id      TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    task_id       TEXT NOT NULL DEFAULT '',
    name          TEXT NOT NULL DEFAULT '',
    remote_path   TEXT NOT NULL DEFAULT '',
    content_type  TEXT NOT NULL DEFAULT 'application/octet-stream',
    size_bytes    BIGINT NOT NULL DEFAULT 0,
    sha256        TEXT NOT NULL DEFAULT '',
    storage_path  TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS loot_agent_created_idx ON loot (agent_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS loot;
ALTER TABLE agents DROP COLUMN IF EXISTS has_desired_sleep;
ALTER TABLE agents DROP COLUMN IF EXISTS desired_jitter_percent;
ALTER TABLE agents DROP COLUMN IF EXISTS desired_sleep_seconds;
ALTER TABLE agents DROP COLUMN IF EXISTS ops;
ALTER TABLE agents DROP COLUMN IF EXISTS features;
ALTER TABLE agents DROP COLUMN IF EXISTS jitter_percent;
ALTER TABLE agents DROP COLUMN IF EXISTS note;
