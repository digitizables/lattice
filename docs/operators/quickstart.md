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

The agent persists its ID under the OS config dir (`%AppData%\lattice\agent.id` on Windows) so restarts reconnect as the same session. Override with `-id-file` or `LATTICE_AGENT_ID_FILE`.

4. List sessions and task:

```text
go run ./client/cmd/client sessions
go run ./client/cmd/client task <agent_id> whoami
go run ./client/cmd/client task <agent_id> echo hello
```

Builtins: `whoami`, `echo`, `hostname`.

Shell tasking (60s timeout):

```text
go run ./client/cmd/client task <agent_id> dir
go run ./client/cmd/client task <agent_id> shell "ipconfig /all"
go run ./client/cmd/client task <agent_id> powershell "Get-Process | Select -First 5"
go run ./client/cmd/client task <agent_id> ls -la
```

On Windows, bare commands and `shell`/`cmd` use `cmd.exe`; `powershell`/`pwsh` use PowerShell. On Unix, bare commands and `shell` use `$SHELL` (fallback `/bin/sh`); `bash` is available when installed.

`client task` prints the task id, then waits until the agent finishes (status lines, then stdout/stderr).

## Web UI

With the teamserver running:

```text
cd client/ui/web
npm install
npm run dev
```

Open http://127.0.0.1:5173. Vite proxies Connect calls to the teamserver on `:8080`.

The UI uses a Secret-style shell: title bar, icon sidebar, sessions list, full-page console, and a listeners status view.

Open a session in **Console**, then type commands and press Enter. The shell starts at `C:\` (Windows) or `/` (Unix). Examples: `pwd`, `cd Windows`, `dir`, `powershell Get-Date`. Local helpers: `help`, `clear`, `sessions`, `use <agent_id>`. ↑/↓ history, Ctrl+L clears, **Ctrl+C** aborts a running task.

**Settings** (gear in the sidebar) → **Terminal** controls full vs partial path in the prompt.

Agent output is capped at 256KiB.

The agent process must be running while you task. If the console stays on a queued status, start:

```text
go run ./agent/cmd/agent -server https://127.0.0.1:8443
```
