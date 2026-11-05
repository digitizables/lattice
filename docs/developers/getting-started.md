# Getting started

Get one full path working before anything else: from the operator through the server and HTTP channel to the agent, then back with a result. By then Compose should be up with Postgres, the teamserver, and nginx for the UI.

## Progress

- **Protocol** (`protocol/lattice/...`, Buf generate into `gen/`): Done
- **Store** (`server/store`, goose + pgx): Done
- **Core** (`server/core`): Done
- **Config** (`server/config`): Done
- **HTTP channel + listen manager**: Done (TLS by default; lab certs under `data/certs` or Compose `/data/certs`)
- **Agent runtime / transport**: Done (builtins + OS shell; persist + uninstall/cleanup; live output + Ctrl+C)
- **Connect API + CLI**: Done (`sessions` paginated/searchable, `task` waits on `StreamEvents`)
- **Scale**: Cursor pagination, server-side search, agent indexes, check-in events only on new sessions
- **Extension loader**: Done (manifest discover + builtin channel factories)
- **Extension install**: Done for local dirs, zip/tar.gz (`-url`), and GitHub zipballs (`-github owner/repo[@ref]`)
- **Web UI**: Done (Vite + Svelte under `client/ui/web`; also served via Compose `web` on `:3000`)
- **Compose teamserver/web images**: Done (`packager/docker/docker-compose.yml`)

### Lab run (Compose)

```text
docker compose -f packager/docker/docker-compose.yml up -d --build
```

- UI: http://127.0.0.1:3000
- Operator API: http://127.0.0.1:8080
- Agent channel: https://127.0.0.1:8443

Lab agent (separate compose file):

```text
docker compose -f packager/docker/docker-compose.lab.yml up -d --build
```

### Lab run (host binaries)

```text
docker compose -f packager/docker/docker-compose.yml up -d postgres

go run ./server/cmd/teamserver
go run ./agent/cmd/agent -server https://127.0.0.1:8443
go run ./client/cmd/client sessions
go run ./client/cmd/client task <agent_id> whoami
```

The agent defaults to `-insecure` so lab self-signed certs work. Set `-insecure=false` when using a real certificate. Agent IDs persist to the OS config dir (or `-id-file`) so restarts do not create duplicate sessions.

Env defaults:

- `LATTICE_DATABASE_URL` = `postgres://lattice:lattice@127.0.0.1:5432/lattice?sslmode=disable`
- `LATTICE_HTTP_ADDR` = `:8080` (Connect operator API)
- `LATTICE_AGENT_ADDR` = `:8443` (HTTPS agent channel)
- `LATTICE_API_URL` = `http://127.0.0.1:8080` (CLI)
- `LATTICE_EXTENSIONS_DIR` = `extensions`
- `LATTICE_CERT_DIR` = `data/certs`
- `LATTICE_AGENT_CLEARTEXT` = `false` (set `true` only to force plain HTTP)

Regenerate stubs after proto edits:

```text
cd protocol
buf dep update
buf generate
cd ..
go mod tidy
```

## Stack

- **Teamserver, CLI, reference agent:** Go (module `go 1.26`)
- **Web UI:** TypeScript, Vite, Svelte (`client/ui/web`)
- **Schemas / codegen:** Protobuf + [Buf](https://buf.build)
- **Operator and server:** Connect v2 (`connectrpc.com/connect/v2` + `connecthttp`)
- **Agent and server (v1 channel):** HTTP(S), Protobuf bodies at `/v1/checkin` and `/v1/result`
- **DB:** PostgreSQL 16
- **Config / profiles:** env for server (`LATTICE_*`); YAML profiles for channels
- **Logs:** `log/slog`
- **Compose services:** `postgres`, `teamserver`, `web`

### Go modules in use

- `google.golang.org/protobuf`
- `connectrpc.com/connect/v2`
- `connectrpc.com/connect/v2/connecthttp`
- `github.com/google/uuid`
- `github.com/jackc/pgx/v5`
- `github.com/pressly/goose/v3`

### Web packages

- `svelte`, `@sveltejs/vite-plugin-svelte`, `vite`, `typescript`
- `marked`, `ansi_up`, `highlight.js`

## Build order

1. **`protocol/`** - done
2. **`server/store`** - done
3. **`server/core`** - done
4. **`extensions/channels/http`** + **`server/listen`** - done
5. **`agent/runtime`** + **`agent/transport`** - done
6. **`server/api`** - done (`StreamEvents` live)
7. **`client` CLI** - done for sessions/task (waits for completion)
8. **`packager/docker`** - done (Postgres + teamserver + web)
9. **Extension loader** - done for in-process channels; remote install later
10. **Web UI** - done (`npm run dev` or Compose `web` on `:3000`)

## Checklist

- [x] Server and agent import Buf-generated Go types
- [x] Compose `postgres` healthy; teamserver/agent/client lab path works locally
- [x] Binaries build (`teamserver`, `agent`, `client`)
- [x] End-to-end check-in + task against live Postgres (`whoami` / `echo` completed in DB)
- [x] HTTP channel loads through the extension loader from `extensions/channels/http/manifest.yaml`
- [x] Compose `teamserver` / `web` images build and run
- [x] Local extension package install (`install-extension -src`)
- [x] Remote extension install from GitHub zipball or release archive (`-github` / `-url`)

Operator docs go in `docs/operators/`. Extension notes: [extensions.md](extensions.md).
