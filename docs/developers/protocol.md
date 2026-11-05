# Protocol

Two Protobuf packages share one Buf config under `protocol/`.

## Operator API (Connect v2)

Package `lattice.operator.v1` — Go Connect handlers under `gen/lattice/operator/v1/operatorv1connect`.

Unary RPCs use `Content-Type: application/json` with `Connect-Protocol-Version: 1` from the web UI. Streaming (`StreamEvents`) needs Connect framed envelopes (`application/connect+json`).

### RPCs

- `ListSessions` / `DeleteSession` — fleet listing (pagination, query, `active_count`)
- `SubmitTask` / `GetTask` / `CancelTask` — tasking
- `ListListeners` / `ReloadExtensions` — channel status and hot reload
- `ListAgentUpdates` / `PublishAgentUpdate` — agent binary catalog
- `ListElevateModules` — BYO elevation catalog
- `StreamEvents` — task (and new-session) event stream

## Agent channel (HTTP Protobuf)

Package `lattice.agent.v1` — not Connect. Bodies are `application/protobuf` on the HTTP channel.

Typical endpoints (HTTP channel):

- `POST /v1/checkin`
- `POST /v1/result`
- `POST /v1/task_running`, `/v1/task_cancel_poll`, `/v1/task_output`
- `POST /v1/update/check`, `/v1/update/download`
- `POST /v1/module/download`

## Codegen

```text
make proto
# or: cd protocol && buf dep update && buf generate
go mod tidy
```

`buf.yaml` / `buf.gen.yaml` use Buf config **v2**. That is the Buf schema version, not the API package version (`v1` packages remain until a wire break).
