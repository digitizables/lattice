package console

import "fmt"

// Usage prints CLI help.
func Usage() {
	fmt.Print(`lattice client

Usage:
  client sessions
  client task <agent_id> <name> [args...]

Env:
  LATTICE_API_URL   teamserver Connect base URL (default http://127.0.0.1:8080)
`)
}
