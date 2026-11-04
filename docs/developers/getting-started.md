# Getting started

Get one full path working before anything else: from the operator through the server and HTTP channel to the agent, then back with a result. By then Compose should be up with Postgres, the teamserver, and nginx for the UI.

## Stack

- **Teamserver, CLI, reference agent:** Go 1.23
- **Web UI:** TypeScript, Vite, Svelte (`client/ui/web`)
- **Schemas / codegen:** Protobuf + [Buf](https://buf.build)
- **Operator and server:** [Connect](https://connectrpc.com) over HTTP/2 (Protobuf). Same services for CLI and browser. Live updates via Connect server streaming.
- **Agent and server (v1 channel):** HTTPS, Protobuf bodies
- **DB:** PostgreSQL 16
- **Config / profiles:** YAML
- **Logs:** `log/slog`
- **Compose services:** `postgres`, `teamserver`, `web` (nginx serves the Vite build)

Keep server, CLI, and the reference agent in Go so `protocol/` only has to generate one Go type set. The UI uses Buf's TypeScript output from those same protos.

### Go modules

- `google.golang.org/protobuf`
- `connectrpc.com/connect`
- `github.com/spf13/cobra`
- `github.com/spf13/viper`
- `github.com/jackc/pgx/v5`
- `github.com/pressly/goose/v3`
- `github.com/go-chi/chi/v5` (agent listeners and other plain HTTP)

### Web packages

- `svelte`, `@sveltejs/vite-plugin-svelte`, `vite`, `typescript`
- Connect TS client from Buf (`protocol/`)

## Build order

1. **`protocol/`** - check-in, task, result messages; Connect services for sessions, tasking, events. Generate Go from Buf now; TypeScript when you start the UI.
2. **`server/store`** - goose migrations (operators, sessions, tasks, results); talk to the DB with pgx.
3. **`server/core`** - session registry + task queue on top of the store.
4. **`extensions/channels/http`** + **`server/listen`** - one HTTPS listener on chi, behind a channel interface.
5. **`agent/runtime`** + **`agent/transport`** - check in, pull work, run something trivial, post results.
6. **`server/api`** - Connect handlers, simple password/token auth, sessions, tasking, event stream.
7. **`client` CLI** - login, sessions, task, start HTTP listener.
8. **`packager/docker`** - Compose for `postgres`, `teamserver`, `web`; teamserver gets a `data/` volume.
9. **Extension loader** - HTTP channel registers via manifest (so the next channel doesn't mean rewriting `core`).
10. **Web UI** - login, sessions, task box, live output over the same Connect services as the CLI.

Then: generate, loot, real multiplayer/RBAC, more channels, modules, installing extensions from elsewhere.

## Checklist

- Server and agent both import the Buf-generated Go types
- `postgres`, `teamserver`, and `web` come up clean in Compose
- Agent check-in leaves a session row
- CLI can task an agent and print the output
- HTTP channel loads as a plugin through the listen interface

Operator docs go in `docs/operators/`. Extension author docs can live with the templates under `extensions/` when those exist.
