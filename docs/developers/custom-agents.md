# Custom agents

How a third-party implant talks to Lattice without forking the reference agent.

Lattice keeps **Protobuf** on the agent channel (not line-delimited JSON). Speak `lattice.agent.v1` over the HTTP(S) channel, or the length-prefixed TCP+TLS channel.

## Minimum viable session

1. Obtain the shared **agent token** from the teamserver (`LATTICE_AGENT_TOKEN` or `data/secrets/agent_token`).
2. `POST /v1/checkin` with `application/protobuf` body `AgentCheckIn`:
   - `hostname`, `username`, `os`, `arch`
   - `agent_token`
   - `features` / `ops` listing what you implement
   - empty `agent_id` on first contact
3. Persist the returned `agent_id`.
4. Run any `Task` values from the response.
5. `POST /v1/result` with `AgentTaskResult` for each task.

Optional: live output (`/v1/task_output`), cancel poll, loot upload/download, update/module download.

## Capability advertisement

Populate `features` (coarse) and `ops` (task names) on every check-in. The operator Session UI surfaces them so buttons and docs stay honest when an agent is partial.

Reference agent features include: `shell`, `ps`, `fs`, `download`, `upload`, `persist`, `update`, `elevate`.

## Structured ops the reference agent implements

- **ps** — process list
- **ls** / **dir** — directory listing
- **cat** / **type** — read file (truncated)
- **download** `<path>` — upload host file into teamserver loot
- **upload** `<loot_id>` `<dest>` — write loot bytes to the host
- **sleep** is operator-driven via `SetAgentSleep` (applied on next check-in response)

Unknown names still fall through to the native shell.

## TCP+TLS channel (optional)

Manifest `extensions/channels/tcp-tls` listens on `:8444` by default (TLS). Framing:

- 1 byte message type
- 4 byte big-endian length
- protobuf body

Types: `1` check-in, `2` check-in response, `3` result, `4` result response.

Prefer HTTPS for the reference agent; use TCP+TLS when you want a reverse-style long-lived socket without HTTP.

## Generate / bake

Operators bake callback settings into a binary:

```text
go run ./server/cmd/generate-agent -server https://host:8443 -sleep 5 -jitter 20 -embed-token
```

Or `OperatorService.GenerateAgent` from the CLI/UI. Ldflags set `agent/bake` variables (`ServerURL`, `AgentToken`, `SleepSeconds`, `JitterPercent`, `InsecureTLS`).

## Auth note

Operator API uses bearer tokens (`Login`). Agents never call OperatorService; they only present `agent_token` on the agent channel.
