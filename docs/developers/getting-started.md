# Getting started

Build a vertical slice before features: one operator action reaches an agent over HTTP and returns a result. Docker Compose runs PostgreSQL, the teamserver, and nginx for the web UI by the end of that slice.

## Stack

- **Language (teamserver, CLI, reference agent):** Go 1.23
- **Operator web UI:** TypeScript, Vite, and Svelte (SPA under `client/ui/web`; not Next.js, not React)
- **Schema and codegen:** Protobuf with [Buf](https://buf.build)
- **Operator ↔ server:** [Connect](https://connectrpc.com) over HTTP/2 (Protobuf). CLI and browser use the same services. Live updates use Connect server streaming.
- **Agent ↔ server (first channel):** HTTPS with Protobuf request and response bodies
- **Database:** PostgreSQL 16 only
- **Profiles and config:** YAML
- **Logging:** Go `log/slog`
- **Containers:** Docker Compose services `postgres`, `teamserver`, and `web` (nginx serving the Vite build)

Server, CLI, and the reference agent stay in Go so `protocol/` generates one set of types. The web app uses Buf-generated TypeScript clients for those same schemas.

### Go modules

- `google.golang.org/protobuf`
- `connectrpc.com/connect`
- `github.com/spf13/cobra`
- `github.com/spf13/viper`
- `github.com/jackc/pgx/v5`
- `github.com/pressly/goose/v3`
- `github.com/go-chi/chi/v5` (agent-facing HTTP listeners and any non-Connect HTTP routes)

### Web packages

- `svelte`, `@sveltejs/vite-plugin-svelte`, `vite`, `typescript`
- Connect TypeScript client generated from `protocol/` via Buf

## Build order

1. **`protocol/`:** `AgentCheckIn`, `Task`, `TaskResult`, plus Connect services for sessions, tasking, and event streams. Generate Go with Buf; generate TypeScript when the UI starts.
2. **`server/store`:** goose migrations for operators, sessions, tasks, and results. Access with pgx.
3. **`server/core`:** Session registry and task queue backed by the store.
4. **`extensions/channels/http`** and **`server/listen`:** One HTTPS listener on chi, behind a channel interface.
5. **`agent/runtime`** and **`agent/transport`:** Check in, pull work, run a trivial command, post Protobuf results.
6. **`server/api`:** Connect handlers, password or token auth stub, session list, task submit, event stream.
7. **CLI (`client`):** Login, list sessions, task an agent, start the HTTP listener.
8. **Compose (`packager/docker`):** `postgres`, `teamserver`, and `web`, with a `data/` volume on the teamserver.
9. **Extension loader:** Register the HTTP channel via manifest so a second channel does not rewrite `core`.
10. **Web UI:** Login, session list, task form, live output through Connect on the same services as the CLI.

After that slice: payload generation, loot, multiplayer RBAC, more channels, modules, and remote extension install.

## Done when

- **Protocol:** Server and agent import the same Buf-generated Go types
- **Compose:** `postgres`, `teamserver`, and `web` come up healthy
- **Beacon:** Agent check-in creates a session row
- **Tasking:** CLI task returns output in the terminal
- **Extensibility:** HTTP channel loads as a plugin behind the listen interface

## Layout pointers

Operator docs belong in `docs/operators/`. Extension authoring details belong next to `extensions/` templates once those exist. Keep this file limited to stack and sequence.
