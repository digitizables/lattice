package console

import "fmt"

// Usage prints CLI help.
func Usage() {
	fmt.Print(`lattice client

Usage:
  client sessions [query]
  client listeners
  client reload-extensions
  client task <agent_id> <name> [args...]

sessions pages through all matches (100 per page).
listeners lists running agent channels.
reload-extensions rediscovers manifests and restarts channels.
task submits work, then waits on StreamEvents until the agent completes it.

Env:
  LATTICE_API_URL   teamserver Connect base URL (default http://127.0.0.1:8080)
`)
}
