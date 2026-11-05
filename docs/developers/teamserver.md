# Teamserver

Entry: `server/cmd/teamserver`.

## Boot sequence

1. Load env config (`server/config`)
2. Open Postgres store and run goose migrations
3. Open updates + modules catalogs
4. Construct `core.Core` (local-agent deny, hostname dedupe purge)
5. Ensure lab agent TLS certs unless `LATTICE_AGENT_CLEARTEXT=true`
6. Discover channel manifests; start enabled channels via listen manager
7. Mount Connect `OperatorService` + `GET /healthz`
8. Listen on `LATTICE_HTTP_ADDR` (default `:8080`)

## Packages

- **`store`** — agents/tasks CRUD, pagination, live task output, cancel flags
- **`core`** — check-in, tasking, events bus, session policy, catalogs
- **`api`** — Connect mapping to core + listen manager + extension runtime
- **`listen`** — register/start/stop/list/reset channels
- **`extensions`** — discover, start, local/remote install, `Runtime.Reload`
- **`updates` / `modules`** — filesystem catalogs for agent binaries and elevate modules
- **`crypto`** — self-signed lab cert for the agent channel
- **`httpx`** — CORS for Vite (`5173`/`4173`) and Compose UI (`3000`)

## Important env

| Variable | Default | Role |
|---|---|---|
| `LATTICE_DATABASE_URL` | local Postgres URL | required |
| `LATTICE_HTTP_ADDR` | `:8080` | Connect API |
| `LATTICE_AGENT_ADDR` | `:8443` | default channel bind |
| `LATTICE_EXTENSIONS_DIR` | `extensions` | manifests root |
| `LATTICE_CERT_DIR` | `data/certs` | agent TLS material |
| `LATTICE_UPDATES_DIR` | `data/updates` | agent update catalog |
| `LATTICE_MODULES_DIR` | `data/modules` | elevate modules |
| `LATTICE_ALLOW_LOCAL_AGENTS` | `false` | allow CC hostname as session |
| `LATTICE_AGENT_CLEARTEXT` | `false` | disable agent TLS (lab only) |

## Publishing artifacts

Large binaries bypass Connect’s size limits via disk tools:

```text
go run ./server/cmd/publish-agent -version 0.2.1 -file ./bin/agent.exe -os windows -arch amd64
go run ./server/cmd/publish-module -name my-uac -version 1.0.0 -os windows -arch amd64 -file ./tool.exe
```
