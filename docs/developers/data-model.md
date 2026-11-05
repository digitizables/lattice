# Data model

PostgreSQL schema is owned by goose migrations under `server/store/migrations/`.

![Agents and tasks](images/data-model.png)

## Tables

### `agents`

Primary implant registry: `id`, host identity fields, `sleep_seconds`, `last_seen`, `created_at`, plus `cwd` (migration `00003`).

### `tasks`

Per-agent work queue/results: `name`, `args[]`, `status`, `exit_code`, `stdout`, `stderr`, timestamps. Cascade-delete with agent.

Statuses used by the product: `queued`, `sent`, `running`, `completed`, `failed`, `cancelled`.

Live streaming updates `stdout`/`stderr` while `status = running`.

### `operators`

Reserved for future operator auth (`username`, `password_hash`). Not wired into the UI yet.

## Indexes

Migration `00002` adds search/list helpers for large fleets (hostname/last_seen oriented indexes).
