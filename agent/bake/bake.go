package bake

// Build-time defaults injected via -ldflags -X (generate-agent / GenerateAgent RPC).
// Empty values mean "use CLI flags / env instead".
var (
	ServerURL     = ""
	AgentToken    = ""
	SleepSeconds  = ""
	JitterPercent = ""
	InsecureTLS   = ""
)
