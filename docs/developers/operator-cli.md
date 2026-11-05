# Operator CLI

Binary: `go run ./client/cmd/client` (or `bin/client.exe`).

Env: `LATTICE_API_URL` (default `http://127.0.0.1:8080`).

## Commands

| Command | Behavior |
|---|---|
| `sessions [query]` | Pages through `ListSessions` (100/page) |
| `listeners` | Prints `ListListeners` |
| `reload-extensions` | Calls `ReloadExtensions`, prints started names |
| `task <agent_id> <name> [args…]` | Opens `StreamEvents`, submits task, waits for completed/failed |

`task` prints the task id, status updates, then stdout/stderr, and exits with the task exit code.
