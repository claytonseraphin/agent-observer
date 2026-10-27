-include .env
POSTGRES_USER ?= observer
POSTGRES_DB   ?= observer

COMPOSE = docker compose -f compose.yaml -f compose.dev.yaml

.PHONY: help dev down logs ps psql reset check test run

help:
	@echo "make dev    build and start the dev stack (proxy + postgres)"
	@echo "make down   stop the stack, keep the database volume"
	@echo "make reset  stop the stack and DELETE the database volume"
	@echo "make logs   follow logs"
	@echo "make ps     show container status"
	@echo "make psql   open a psql shell in the database"
	@echo "make check  gofmt + vet + build (same checks CI will run)"
	@echo "make test   run go tests"
	@echo "make run    run the proxy natively (no Docker)"

dev:
	$(COMPOSE) up --build

down:
	$(COMPOSE) down

reset:
	$(COMPOSE) down -v

logs:
	$(COMPOSE) logs -f

ps:
	$(COMPOSE) ps

psql:
	$(COMPOSE) exec postgres sh -c 'psql -U "$$POSTGRES_USER" -d "$$POSTGRES_DB"'

check:
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed on:"; gofmt -l .; exit 1)
	go vet ./...
	go build ./...

test:
	go test -race ./...

run:
	DATABASE_URL='postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@localhost:5432/$(POSTGRES_DB)?sslmode=disable' go run ./cmd/proxy
