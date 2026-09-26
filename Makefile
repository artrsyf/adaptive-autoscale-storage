.DEFAULT_GOAL := help

GO ?= go
DOCKER ?= docker
PWSH ?= pwsh
SCENARIO ?= constant
DISTRIBUTION ?= uniform
RATE ?= 50
SECONDS ?= 60
PARTITIONING_CONFIG ?= config.yaml

# Serialize checks and Compose operations even when make is invoked with -j.
.NOTPARALLEL:
.PHONY: help build test test-race vet fmt fmt-check check test-all integration smoke smoke-failure up down status logs bench partitioning

help:
	@echo "make test          - Go tests in all four modules (no Docker)"
	@echo "make check         - Formatting, build, vet and tests in all modules"
	@echo "make test-all      - check + Docker race/integration tests + public API smoke"
	@echo "make test-race     - Host race tests; requires CGO and a C compiler"
	@echo "make integration   - Docker race tests for processing unit and Router"
	@echo "make smoke         - Start/rebuild stack and check public CRUD API"
	@echo "make smoke-failure - smoke plus PostgreSQL stop/recovery check"
	@echo "make up/down       - Start/rebuild or stop stack, preserving volumes"
	@echo "make status/logs   - Inspect Compose services"
	@echo "make fmt           - Format Go code in all modules"
	@echo "make bench SCENARIO=constant DISTRIBUTION=hot RATE=50 SECONDS=60"
	@echo "make partitioning  - Compare partition placement algorithms and write a local report"

build:
	$(GO) -C processing-unit build ./...
	$(GO) -C router build ./...
	$(GO) -C benchmarks/load build ./...
	$(GO) -C benchmarks/partitioning build ./...

test:
	$(GO) -C processing-unit test ./...
	$(GO) -C router test ./...
	$(GO) -C benchmarks/load test ./...
	$(GO) -C benchmarks/partitioning test ./...

test-race:
	$(GO) -C processing-unit test -race ./...
	$(GO) -C router test -race ./...
	$(GO) -C benchmarks/load test -race ./...
	$(GO) -C benchmarks/partitioning test -race ./...

vet:
	$(GO) -C processing-unit vet ./...
	$(GO) -C router vet ./...
	$(GO) -C benchmarks/load vet ./...
	$(GO) -C benchmarks/partitioning vet ./...

fmt:
	$(GO) -C processing-unit fmt ./...
	$(GO) -C router fmt ./...
	$(GO) -C benchmarks/load fmt ./...
	$(GO) -C benchmarks/partitioning fmt ./...

fmt-check:
	$(PWSH) -NoProfile -Command "$$ErrorActionPreference = 'Stop'; $$files = Get-ChildItem processing-unit,router,benchmarks/load,benchmarks/partitioning -Recurse -Filter *.go; $$unformatted = & gofmt -l $$files.FullName; if ($$LASTEXITCODE -ne 0) { exit $$LASTEXITCODE }; if ($$unformatted) { $$unformatted; exit 1 }"

check: fmt-check build vet test

# Integration containers use -race -count=1 and the real Compose PostgreSQL.
# Smoke starts the current application images after the integration checks.
test-all: check integration smoke

integration:
	$(DOCKER) compose --profile test run --build --rm tests
	$(DOCKER) compose --profile test run --build --rm router-tests

up:
	$(DOCKER) compose up -d --build --wait --remove-orphans

down:
	$(DOCKER) compose down

status:
	$(DOCKER) compose ps

logs:
	$(DOCKER) compose logs --tail 100 router processing-unit-1 processing-unit-2 processing-unit-3 postgres

smoke: up
	$(PWSH) -NoProfile -File benchmarks/smoke.ps1

# Run separately from benchmarks: this deliberately stops PostgreSQL briefly.
smoke-failure: up
	$(PWSH) -NoProfile -File benchmarks/smoke.ps1 -FailureChecks

bench:
	$(PWSH) -NoProfile -File benchmarks/run.ps1 -Scenario $(SCENARIO) -Distribution $(DISTRIBUTION) -Rate $(RATE) -Seconds $(SECONDS)

partitioning:
	$(GO) -C benchmarks/partitioning run ./cmd/partitioning -config $(PARTITIONING_CONFIG) -output ../../results/partitioning
