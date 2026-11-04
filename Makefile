.PHONY: proto tidy compose-up compose-down

proto:
	cd protocol && buf dep update && buf generate

tidy:
	go mod tidy

compose-up:
	docker compose -f packager/docker/docker-compose.yml up -d --build

compose-down:
	docker compose -f packager/docker/docker-compose.yml down
