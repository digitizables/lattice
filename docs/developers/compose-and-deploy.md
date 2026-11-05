# Compose and deploy

## Full stack

```text
docker compose -f packager/docker/docker-compose.yml up -d --build
```

| Service | Ports | Role |
|---|---|---|
| `postgres` | `5432` | Database |
| `teamserver` | `8080`, `8443` | Connect API + agent TLS channel |
| `web` | `3000` | nginx UI + API proxy |

Images: `packager/docker/Dockerfile.teamserver`, `Dockerfile.web`. Teamserver image copies `extensions/channels` and regenerates `gen/` if missing.

## Lab agent

```text
docker compose -f packager/docker/docker-compose.lab.yml up -d --build
```

Hostname `lab-agent`, `restart: unless-stopped`, volume for `agent.id`, targets `host.docker.internal:8443` with `-insecure`.

## Host binaries

Still supported for day-to-day iteration: run Postgres via Compose, then `go run ./server/cmd/teamserver` and agents/CLI on the host.
