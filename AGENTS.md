# Repository Guidelines

## Project Structure & Module Organization

Read `docs/first-epic-contract.md` and the canonical roadmap updates. Earlier roadmap sections retain superseded sequencing. Document decisions and rationale without references to discussions or approvals.

Application entry point: `cmd/storage/`; composition: `internal/app/`. Domain slices live in `internal/document/{domain,service,repository,transport}/`; assignment is separate. Tests are colocated. The independent benchmark Go module and image live in `benchmarks/load/`. Initial SQL: `migrations/`. Experiments: `benchmarks/`. Deployment and dashboards: `compose.yaml` and `deploy/`. Generated results belong in ignored `results/`.

## Architecture & Scope

Go processing units (PUs) use authoritative PostgreSQL storage. Docker Compose provides a local baseline with load tests, Prometheus, and provisioned Grafana dashboards.

Keep domain models and concrete use-case services independent of HTTP, PostgreSQL, and Prometheus. Define narrow dependency interfaces at their consumers; repository ports describe atomic persistence guarantees. Keep wire DTOs in transport adapters. Never import application internals from benchmark clients or run tests/workloads from application startup. Future Scala integrations use explicit contracts. Logical ownership is not physical sharding. Defer Kafka, Redis, Kubernetes, and autoscaling until their planned stages.

## Build, Test, and Development Commands

Use the pinned Go toolchain and Docker Desktop with Linux containers:

- `go build ./...`: compile Go packages.
- `go test ./...`: run Go tests.
- `go vet ./...`: check common correctness issues.
- `go -C benchmarks/load test ./...`: test the independent workload client.
- `docker compose up -d --build --wait`: start the stack after copying `.env.example` to `.env`.
- `docker compose --profile test run --build --rm tests`: run race and PostgreSQL integration tests.
- `./benchmarks/run.ps1 -Scenario constant -Rate 100`: run and archive an experiment.
- `docker compose down`: stop the stack; avoid `-v` unless deleting persisted data is intended.

## Coding Style & Naming Conventions

Use `gofmt` for Go formatting, idiomatic package names, and descriptive exported identifiers. Name Go tests `*_test.go` with `TestXxx` functions. Use versioned, ordered migration filenames. Keep metric labels bounded; never label metrics with record IDs, raw SQL, or arbitrary URLs.

## Testing Guidelines

Use Go's standard testing package; no coverage threshold is established. Integration tests require `TEST_DATABASE_URL` and otherwise skip. Test routing, concurrent writes, cancellation, and database failures. Record benchmark settings, limits, warm-up, throughput, errors, and latency. Measure pool acquisition separately from database execution. Never report unexecuted benchmarks as results.

## Commit & Pull Request Guidelines

Use `epic/<number>-<short-name>` branches. Commit messages include feature number and name, e.g. `docs: Feature 0.1 — Formal system model`. Commit completed work to the epic branch for review. Explain the implementation, answer questions, and address requested changes. Merge into `master` only after checks pass and the user explicitly approves the merge following review. Never infer merge approval from permission to implement or commit. PRs describe behavior, validation, limitations, and dashboard screenshots. Never commit real credentials.

## Local Execution & Design Decisions

Before starting the stack, ask the user to enable WSL2. Calibrate explicit resource limits experimentally; smaller limits alone do not ensure representative results. Verify generator and monitoring headroom. Scala DSL deployment remains an open decision.
