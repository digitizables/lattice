# Development workflow

## Everyday loop

1. Edit protos only when the wire contract changes → `make proto`
2. Iterate Go packages with `go test ./server/...` and targeted `go run`
3. Iterate UI with `npm run dev` in `client/ui/web`
4. Smoke against Compose when changing listen/API/agent paths

## Make targets

| Target | Action |
|---|---|
| `make proto` | Buf generate |
| `make tidy` | `go mod tidy` |
| `make compose-up` | Build/start postgres+teamserver+web |
| `make compose-down` | Stop Compose stack |
| `make test` | Unit tests for server/agent/client |
| `make test-e2e` | Live smoke (`LATTICE_E2E=1`) |

## Extension packages

```text
go run ./server/cmd/install-extension -src ./path/to/channel
go run ./server/cmd/install-extension -url ./channel.zip
go run ./server/cmd/install-extension -github owner/repo@v1.0.0 -subdir path/to/channel
go run ./client/cmd/client reload-extensions
```

## Diagrams for docs

Developer doc figures are exported from FigJam board `pgKRBDPTfSTomv34YhCNy3` into `docs/developers/images/`. When refreshing a figure: generate in that board → screenshot → save PNG → delete the board nodes → leave the file for the next export.
