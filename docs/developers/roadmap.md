# Product roadmap (tracked)

Factual tracker for work inspired by lab C2 patterns (auth, agent token, generate, capabilities, structured ops, loot, multi-channel). Status values: **Done**, **Next**, **Later**.

Update this file when a slice lands. Developer docs describe what exists; this file is the ordered backlog.

## Done (this slice)

- **Operator auth** — Login/Logout, bcrypt operators, bearer tokens, bootstrap admin, login rate limit, HTTP gate on OperatorService (`LATTICE_AUTH_DISABLED` escape)
- **Agent shared token** — `LATTICE_AGENT_TOKEN` or `data/secrets/agent_token`; required on check-in when set
- **Capability advertisement** — check-in `features` / `ops` stored on Session
- **Sleep / jitter** — `SetAgentSleep`; applied via check-in response; agent jittered sleep
- **Structured ops** — `ps`, `ls`/`dir`, `cat`/`type`, `download`, `upload`
- **Loot store** — `/v1/loot/upload|download`, Postgres + `data/loot`, List/Get/DeleteLoot
- **Session notes** — `SetSessionNote`
- **Generate / bake** — `server/cmd/generate-agent`, `GenerateAgent` RPC, `agent/bake` ldflags
- **Custom agent docs** — `docs/developers/custom-agents.md`
- **TCP+TLS channel** — `extensions/channels/tcp-tls` length-prefixed Protobuf on `:8444`

## Next

- **RBAC roles** (admin vs operator vs read-only) beyond single authenticated operator class
- **Audit log** of Login, SubmitTask, ReloadExtensions, Generate
- **Chunked loot / large generate** without Connect body size pain
- **Named-pipe / SMB channel** (Windows-only)
- **Out-of-process / WASM channel runtimes** (drop compile-time factory requirement)
- **Public extension registry**
- **Malleable HTTP profiles** beyond `profiles/http-default.yaml`
- **Multiplayer presence** in the web console

## Later

- Pivot / peer graph
- HA Postgres and retention jobs
- Signed release artifacts and SECURITY.md threat model

## Deliberately not in Lattice core

- PE scan / decompiler / debugger / bindiff / fuzz workbench (separate RE tools)
- Default inject/telemetry hooks as stock agent features
- Line-delimited JSON agent wire (Lattice stays Protobuf + Connect)
