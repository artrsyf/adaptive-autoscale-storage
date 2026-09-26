# Repository Guidelines

## Project Structure & Module Organization

Read `docs/README.md`, `docs/roadmap.md`, and `docs/epics/epic-1/README.md`. Read `docs/epics/epic-1/changelog.md` for branch history and the continuation checklist. Keep current architecture, research plans, and validation evidence distinct. Document decisions and rationale without references to discussions or approvals.

Separate entry points: `processing-unit/cmd/processing-unit/` and `router/cmd/router/`; composition: `processing-unit/internal/app/` and `router/internal/app/`. Application-private code lives under each application’s `internal/`; each application owns its models, HTTP adapters and utilities; only Router owns topology and partition hashing; `docs/epics/epic-1/api.md` defines network compatibility. Tests are colocated. The independent benchmark Go module and image live in `benchmarks/load/`. Initial SQL: `processing-unit/migrations/`. Experiments: `benchmarks/`. Deployment and dashboards: `docker-compose.yaml` and `deploy/`. Generated results belong in ignored `results/`. Application settings: `processing-unit/config.yaml` and `router/config.yaml`; workload settings: `benchmarks/load/config.yaml`. Infrastructure uses native Compose/Prometheus/Grafana files. Only passwords belong in `.env`.

## Architecture & Scope

Go processing units use authoritative PostgreSQL storage. Docker Compose provides a local baseline with load tests, Prometheus, and provisioned Grafana dashboards.

Keep domain models and concrete use-case services independent of HTTP, PostgreSQL, and Prometheus. Define narrow dependency interfaces at their consumers; repository ports describe atomic persistence guarantees. Keep wire DTOs in transport adapters. Transport converts DTOs to entities and invokes services. Routing decisions, processing unit destination/epoch/partition-range checks and command admission belong in services. Never merge Router and processing unit through a role flag or add PostgreSQL dependencies to Router. Never import application internals from benchmark clients or run tests/workloads from application startup. Future Scala integrations use explicit contracts. Logical ownership is not physical sharding. Defer Kafka, Redis, Kubernetes, and autoscaling until their planned stages.

## Build, Test, and Development Commands

Use the pinned Go toolchain and Docker Desktop with Linux containers. Root Makefile shortcuts: `make test` runs all three modules; `make check` adds formatting/build/vet; `make test-all` adds Docker race/PostgreSQL integration tests and a live CRUD smoke check. These targets require GNU Make and PowerShell 7. Docker targets leave the stack running and preserve volumes. Failure injection, benchmarks and visual checks remain separate. Direct commands:

- `go -C processing-unit build ./...` (also run with `-C router`): compile Go packages.
- `go -C processing-unit test ./...` (also run with `-C router`): run Go tests.
- `go -C processing-unit vet ./...` (also run with `-C router`): check common correctness issues.
- `go -C benchmarks/load test ./...`: test the independent workload client.
- `docker compose up -d --build --wait --remove-orphans`: build/start the configured stack; initialize `.env` from the example once.
- `docker compose --profile test run --build --rm tests`: run race and PostgreSQL integration tests.
- `docker compose --profile test run --build --rm router-tests`: run Router race tests.
- `./benchmarks/run.ps1 -Scenario constant -Rate 100`: run and archive an experiment.
- `docker compose down`: stop the stack; avoid `-v` unless deleting persisted data is intended.

## Coding Style & Naming Conventions

Use `gofmt` for Go formatting, idiomatic package names, and descriptive exported identifiers. Give every receiver, parameter, local variable and test variable a meaningful name; do not use single-letter names. Prefer lower camel case matching the type or role: `documentService`, `documentCommand`, `documentHttpConfig`, `httpRequest`, `responseWriter`, `testCase`, `recordIndex`. Apply the same rule to benchmark and deployment scripts. Name Go tests `*_test.go` with `TestXxx` functions. Use versioned, ordered migration filenames. Keep metric labels bounded; never label metrics with record IDs, raw SQL, or arbitrary URLs.

Keep component Config types and metrics with their consuming adapters. Use descriptive names such as `DocumentPostgresConfig` and `DocumentPostgresRepository`. Technical helpers belong in each application’s `internal/utils`. Keep applications as independent Go modules without cross-application imports or local replacements. Prefer native YAML configuration and small explicit duplication over generators or infrastructure schemas in Go. Keep duplicated addresses consistent manually.

## Testing Guidelines

Use Go's standard testing package; no coverage threshold is established. PostgreSQL tests use the application YAML and `POSTGRES_PASSWORD`; use `docker compose --profile test run --build --rm tests`. Test routing, concurrent writes, cancellation, and database failures. Record benchmark settings, limits, warm-up, throughput, errors, and latency. Measure pool acquisition separately from database execution. Never report unexecuted benchmarks as results.

## Commit & Pull Request Guidelines

Use `epic/<number>-<short-name>` branches. Commit messages include feature number and name, e.g. `docs: Feature 0.1 — Formal system model`. Leave review changes uncommitted until the user explicitly requests a commit. Explain the implementation, answer questions, and address requested changes. Merge into `master` only after checks pass and the user explicitly approves the merge following review. Never infer merge approval from permission to implement or commit. PRs describe behavior, validation, limitations, and dashboard screenshots. Never commit real credentials.

## Local Execution & Design Decisions

Before starting the stack, ask the user to enable WSL2. Calibrate explicit resource limits experimentally; smaller limits alone do not ensure representative results. Verify generator and monitoring headroom. Scala DSL deployment remains an open decision.

## Naming & Slice Layout

Organize application functionality as `internal/<entity>/{domain,service,repository/<technology>,transport/<protocol>,client/<protocol>}`; create only the layers used. No generic root `internal/client` or `internal/service`. Name services, ports, adapters and configs after their entity: `DocumentService`, `DocumentRepository`, `DocumentPostgresRepository`, `DocumentHttpHandler`, `ProcessingUnitHttpClient`. Use `snake_case` filenames matching the primary type, with `_test.go` for tests. Keep consumer ports beside their service. Small domain value types use matching filenames (`Key` in `key.go`). Assignment is `assignment/service/StaticAssignmentService`, with separate `AssignmentConfig` and route models under `assignment/domain`. Shared technical helpers remain under `internal/utils`.

Name utility files after their concrete operation or type: `config_decoder.go` for strict YAML decoding, `duration.go` for YAML durations, `http_server.go` for HTTP lifecycle. Avoid generic names such as `utils/config.go`.

Place all wire DTOs in each functionality’s `transport/dto`, separate from `transport/http`. Use operation-specific types and matching files, e.g. `DocumentCreateRequest` in `document_create_request.go` and `DocumentCreateResponse` in `document_create_response.go`. Avoid universal request/response structs spanning different operations. Independent clients own their DTOs and declare only operations they use.

## Transport Architecture

Apply this structure to every application and every new entity, not only documents:

- One `<Entity>HttpHandler` structure per entity and application. Keep operation methods together in `<entity>_http_handler.go`, e.g. `DocumentHttpHandler.CreateDocument`, `GetDocument`, `UpdateDocument`, `DeleteDocument`. Do not create a handler structure or file per operation.
- Register method/path pairs explicitly in `<entity>_http_routes.go`, binding routes directly to handler methods. Do not parse paths or switch on operation names in a universal inbound handler.
- Each method decodes its operation-specific DTO, converts it to domain input, calls the application service, and returns the matching response DTO. Business decisions remain in services.
- Reuse one `<Entity>HttpConfig` and common middleware for deadlines, body limits, error responses and HTTP metrics across the entity's endpoints. Avoid per-operation copies of these settings and helpers.
- Keep operation-specific request/response DTOs in `transport/dto` with their existing descriptive filenames. Consolidating handlers does not mean merging DTOs.
- Register technical health and metrics endpoints in server infrastructure; do not invent domain slices or per-endpoint structures for them. External benchmark clients keep their own transport DTOs and do not import application internals.

Router and processing unit each own their `DocumentHttpHandler`; they remain independent applications. An application service may accept a domain command through `Execute` for routing/admission, but it must not dispatch HTTP requests.
