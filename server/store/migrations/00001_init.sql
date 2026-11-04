-- +goose Up
CREATE TABLE agents (
    id              TEXT PRIMARY KEY,
    hostname        TEXT NOT NULL DEFAULT '',
    username        TEXT NOT NULL DEFAULT '',
    os              TEXT NOT NULL DEFAULT '',
    arch            TEXT NOT NULL DEFAULT '',
    sleep_seconds   BIGINT NOT NULL DEFAULT 0,
    last_seen       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE tasks (
    id           TEXT PRIMARY KEY,
    agent_id     TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    args         TEXT[] NOT NULL DEFAULT '{}',
    status       TEXT NOT NULL DEFAULT 'queued',
    exit_code    INTEGER,
    stdout       TEXT NOT NULL DEFAULT '',
    stderr       TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX tasks_agent_status_idx ON tasks (agent_id, status);

CREATE TABLE operators (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS agents;
DROP TABLE IF EXISTS operators;
