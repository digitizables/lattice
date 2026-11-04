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

2. Start the teamserver (operator API on `:8080`, agent channel on `:8443`):

```text
go run ./server/cmd/teamserver
```

3. Start an agent:

```text
go run ./agent/cmd/agent -server http://127.0.0.1:8443
```

4. List sessions and task:

```text
go run ./client/cmd/client sessions
go run ./client/cmd/client task <agent_id> whoami
go run ./client/cmd/client task <agent_id> echo hello
```

Builtins today: `whoami`, `echo`, `hostname`.

Results are stored when the agent posts them. The CLI does not yet stream live task output (`StreamEvents` is unimplemented).
