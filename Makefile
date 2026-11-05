.PHONY: proto tidy compose-up compose-down test test-e2e

proto:
	cd protocol && buf dep update && buf generate

tidy:
	go mod tidy

compose-up:
	docker compose -f packager/docker/docker-compose.yml up -d --build

compose-down:
	docker compose -f packager/docker/docker-compose.yml down

test:
	go test ./server/... ./agent/... ./client/...

# Requires a live stack (compose teamserver + lab agent).
test-e2e:
	LATTICE_E2E=1 go test -tags e2e ./test/e2e -count=1 -timeout 3m
