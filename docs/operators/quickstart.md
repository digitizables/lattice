# Operator quickstart

## Prerequisites

- Go toolchain
- PostgreSQL 16 (Compose service `postgres` is fine)
- Built or `go run` access to `teamserver`, `agent`, and `client`

## Start the stack

1. Start the database:

```text
docker compose -f packager/docker/docker-compose.yml up -d postgres
```

2. Start the teamserver (operator API on `:8080`, HTTPS agent channel on `:8443`):

```text
go run ./server/cmd/teamserver
```

Lab TLS certs are created under `data/certs/` on first start.

3. Start an agent:

```text
go run ./agent/cmd/agent -server https://127.0.0.1:8443
```

4. List sessions and task:

```text
go run ./client/cmd/client sessions
go run ./client/cmd/client task <agent_id> whoami
go run ./client/cmd/client task <agent_id> echo hello
```

Builtins today: `whoami`, `echo`, `hostname`.

`client task` prints the task id, then waits on `StreamEvents` until the agent finishes (status lines, then stdout/stderr).

## Web UI

With the teamserver running:

```text
cd client/ui/web
npm install
npm run dev
```

Open http://127.0.0.1:5173. Vite proxies Connect calls to the teamserver on `:8080`.

The UI follows the Secret-style shell from the Lattice Figma file (`C2 · Terminal Emulator`): title bar, icon sidebar, folder tabs, split sessions/task panes, and a bottom terminal dock for live task output.
