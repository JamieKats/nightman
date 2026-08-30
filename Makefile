# Nightman — task runner.
# Skeleton: the Go basics work now; infra targets are stubs that get
# filled in as the corresponding milestones land (see docs/IMPLEMENTATION_PLAN.md).

BINARY  := nightman
CMD     := ./cmd/nightman
BIN_DIR := bin

.DEFAULT_GOAL := help

## help: list available targets
.PHONY: help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'

# ---------------------------------------------------------------------------
# Build & run
# ---------------------------------------------------------------------------

## build: compile the binary into ./bin
.PHONY: build
build:
	go build -o $(BIN_DIR)/$(BINARY) $(CMD)

## run: run the honeypot from source
.PHONY: run
run:
	go run $(CMD)

## clean: remove build artifacts
.PHONY: clean
clean:
	rm -rf $(BIN_DIR)

# ---------------------------------------------------------------------------
# Quality
# ---------------------------------------------------------------------------

## test: run the test suite
.PHONY: test
test:
	go test ./...

## fmt: format all Go source
.PHONY: fmt
fmt:
	gofmt -l -w .

## vet: run go vet
.PHONY: vet
vet:
	go vet ./...

## lint: run golangci-lint (requires golangci-lint on PATH)
.PHONY: lint
lint:
	golangci-lint run

## tidy: sync go.mod / go.sum
.PHONY: tidy
tidy:
	go mod tidy

# ---------------------------------------------------------------------------
# Local infra — TODO: implemented alongside Phase 3 (persistence)
# ---------------------------------------------------------------------------

## up: start local dependencies (Postgres, Grafana) via docker compose
.PHONY: up
up:
	@echo "TODO: docker compose up -d  (needs docker-compose.yml — Phase 3, step 12)"

## down: stop local dependencies
.PHONY: down
down:
	@echo "TODO: docker compose down"

## migrate: apply SQL migrations in ./migrations
.PHONY: migrate
migrate:
	@echo "TODO: apply migrations/*.sql via psql  (Phase 3, step 9)"
