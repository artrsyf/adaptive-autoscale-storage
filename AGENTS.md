# Repository Guidelines

## Project Structure & Module Organization

The repository currently contains design and roadmap documents in `docs/`; application code, tests, and deployment configuration do not yet exist. Read `docs/first-epic-contract.md` for the initial scope and implementation plan, then the canonical roadmap updates; earlier sections retain superseded sequencing. Document selected decisions and their rationale without references to user discussions or approvals.

As implementation begins, organize Go entry points under `cmd/`, private packages under `internal/`, database migrations under `migrations/`, and load scenarios under `benchmarks/`. Keep deployment and observability configuration under `deploy/`. These are proposed conventions, not existing directories.

## Architecture & Scope

The planned data plane uses Go processing units (PUs) backed by authoritative PostgreSQL storage. The immediate priority is a locally runnable Docker Compose baseline with load tests, Prometheus metrics, and provisioned Grafana dashboards showing load distribution and database pressure.

Preserve boundaries between request handling, routing, partition assignment, and persistence. Future Scala control-plane and query-DSL integrations should use explicit contracts. Logical partition ownership does not imply physical PostgreSQL sharding. Defer Kafka, Redis, Kubernetes, and automatic scaling until their planned stages.

## Build, Test, and Development Commands

No build or test commands are currently configured. Once the Go module and Compose configuration exist, document and verify these expected commands:

- `go build ./...`: compile Go packages.
- `go test ./...`: run Go tests.
- `go vet ./...`: check common correctness issues.
- `docker compose up --build`: build and start the local stack.
- `docker compose down`: stop the stack; avoid `-v` unless deleting persisted data is intended.

## Coding Style & Naming Conventions

Use `gofmt` for Go formatting, idiomatic package names, and descriptive exported identifiers. Name Go tests `*_test.go` with `TestXxx` functions. Use versioned, ordered migration filenames. Keep metric labels bounded; never label metrics with record IDs, raw SQL, or arbitrary URLs.

## Testing Guidelines

Use Go's standard testing package; no framework or coverage threshold is established. Test routing, concurrent writes, cancellation, and database failures as implemented. Separate integration tests from reproducible load scenarios. Record workload settings, resource limits, warm-up, throughput, errors, and latency. Measure pool acquisition separately from database execution; never report unexecuted benchmarks as results.

## Commit & Pull Request Guidelines

Use one branch per epic, named `epic/<number>-<short-name>`. Commit changes within that branch and include the feature number and name, for example `docs: Feature 0.1 — Formal system model`. Merge into `master` only after every epic requirement and its validation are complete. PRs should explain behavior, scope, validation results, and unresolved limitations. Include dashboard screenshots for visualization changes. Keep credentials out of tracked configuration.

## Local Execution & Design Decisions

Before starting the local stack, ask the user to enable WSL2. Use modest, explicit resource limits and calibrate load experimentally; smaller limits alone do not establish representative results. Ensure the load generator and monitoring stack have sufficient headroom. The initial document model and conditional writes are recorded in `docs/first-epic-contract.md`. Future Scala DSL deployment remains an open decision.
