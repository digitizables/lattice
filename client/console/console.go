package console

import "fmt"

// Usage prints CLI help.
func Usage() {
	fmt.Print(`lattice client

Usage:
  client login <user> <pass>
  client logout
  client sessions [query]
  client listeners
  client reload-extensions
  client task <agent_id> <name> [args...]
  client note <agent_id> <text...>
  client sleep <agent_id> <seconds> [jitter_percent]
  client loot [agent_id]
  client generate-agent [flags]

sessions pages through all matches (100 per page) and shows note/features when set.
listeners lists running agent channels.
reload-extensions rediscovers manifests and restarts channels.
task submits work, then waits on StreamEvents until the agent completes it.
note sets the operator session note.
sleep queues sleep/jitter for the next agent check-in.
loot lists loot metadata (optionally filtered by agent).
generate-agent calls GenerateAgent (requires LATTICE_OPERATOR_TOKEN); flags:
  -server -os -arch -sleep -jitter -insecure -embed-token -o
  Without a token, use: go run ./server/cmd/generate-agent

Env:
  LATTICE_API_URL          teamserver Connect base URL (default http://127.0.0.1:8080)
  LATTICE_OPERATOR_TOKEN   bearer token from login (Authorization: Bearer ...)
`)
}
