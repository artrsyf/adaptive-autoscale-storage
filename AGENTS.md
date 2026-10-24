# Repository Guidelines

## Project Structure & Module Organization

Read `docs/README.md`, `docs/roadmap.md`, and `docs/epics/epic-1/README.md`. Read `docs/epics/epic-1/changelog.md` for branch history and the continuation checklist. Keep current architecture, research plans, and validation evidence distinct. Document decisions and rationale without references to discussions or approvals.

Separate entry points: `processing-unit/cmd/processing-unit/` and `router/cmd/router/`; composition: `processing-unit/internal/app/` and `router/internal/app/`. Application-private code lives under each application’s `internal/`; each application owns its models, HTTP adapters and utilities; only Router owns topology and partition hashing; `docs/epics/epic-1/api.md` defines network compatibility. Tests are colocated. The independent benchmark Go module and image live in `benchmarks/load/`. Initial SQL: `processing-unit/migrations/`. Experiments: `benchmarks/`. Compose deployment and dashboards live under `docker-compose.yaml` and `deploy/`; the k3s baseline is the application Helm chart rooted at `deploy/kubernetes/`. Organize chart resources vertically by deployed component: each component directory owns its workload, Service and component-specific RBAC; keep exactly one Kubernetes resource per source manifest. Shared Helm templates live in `templates/`, while only genuinely shared namespace and access resources belong in `foundation/` and `access/`. The suspended load CronJob is a template for explicit Jobs; its generator remains running after a run until artifacts are copied and the exact Job is deleted. Generated results belong in ignored `results/`. Application settings: `processing-unit/config.yaml` and `router/config.yaml`; workload settings: `benchmarks/load/config.yaml`. Infrastructure uses native Compose, Helm, Kubernetes, Prometheus and Grafana files. Only passwords belong in `.env`.

## Architecture & Scope

Go processing units use authoritative PostgreSQL storage. Docker Compose provides the fast local regression baseline; the existing WSL2 k3s cluster provides the static Kubernetes baseline with load Jobs, Prometheus and provisioned Grafana dashboards.

Keep domain models and concrete use-case services independent of HTTP, PostgreSQL, Prometheus and Kubernetes. Define narrow dependency interfaces at their consumers; repository ports describe atomic persistence guarantees. Keep wire DTOs in transport adapters. Transport converts DTOs to entities and invokes services. Routing decisions, processing unit destination/epoch/partition-range checks and command admission belong in services. Never merge Router and processing unit through a role flag or add PostgreSQL dependencies to Router. Never import application internals from benchmark clients or run tests/workloads from application startup. Future Scala integrations use explicit contracts. Logical ownership is not physical sharding. Kubernetes resource types, clients, Pod identity and replica state remain in infrastructure adapters; current application workloads do not call the Kubernetes API. Defer Kafka, Redis and autoscaling actuators until their planned stages.

Router is the only public application entrypoint. Publish only its documented business routes through a TLS-terminating Ingress; keep operational endpoints and all other Services cluster-internal. Local k3s may use a generated self-signed certificate stored under ignored `results/`; a non-local environment must provide the same TLS Secret contract from a trusted issuer and replace the local hostname through an environment-specific deployment overlay. Ingress does not substitute for future application authentication, authorization, tenant isolation or abuse controls.

Helm values contain only existing Secret names, never secret payloads. Local secret material is created by `k3s.ps1` before Helm runs; production must provision the same Secret contracts externally. Assignment bootstrap is a regular chart Job during initial install so Router readiness cannot deadlock Helm hook scheduling, and a `post-upgrade` hook on upgrades. Pass canonical application configs, migrations, exporter queries and dashboards with `--set-file` instead of copying them into the chart. Keep checksum annotations on consuming Pod templates so Helm upgrades roll workloads when supplied files change. The first Helm adoption may remove only explicitly recognized hashed ConfigMaps left by the former Kustomize deployment; never apply broad ConfigMap cleanup patterns.

## Build, Test, and Development Commands

Use the pinned Go toolchain and Docker Desktop with Linux containers. Root Makefile shortcuts: `make test` runs all four modules; `make check` adds formatting/build/vet; `make test-all` adds Docker race/PostgreSQL integration tests and a live CRUD smoke check. These targets require GNU Make and PowerShell 7. Docker targets leave the stack running and preserve volumes. Failure injection, benchmarks and visual checks remain separate. Direct commands:

- `go -C processing-unit build ./...` (also run with `-C router`): compile Go packages.
- `go -C processing-unit test ./...` (also run with `-C router`): run Go tests.
- `go -C processing-unit vet ./...` (also run with `-C router`): check common correctness issues.
- `go -C benchmarks/load test ./...`: test the independent workload client.
- `go -C benchmarks/partitioning test ./...`: test the independent partitioning research harness.
- `docker compose up -d --build --wait --remove-orphans`: build/start the configured stack; initialize `.env` from the example once.
- `docker compose --profile test run --build --rm tests`: run race and PostgreSQL integration tests.
- `docker compose --profile test run --build --rm router-tests`: run Router race tests.
- `./benchmarks/run.ps1 -Scenario constant -Rate 100`: run and archive an experiment.
- `docker compose down`: stop the stack; avoid `-v` unless deleting persisted data is intended.
- `make k3s-up`: build/import local application images and apply the static baseline to the existing k3s cluster.
- `make k3s-status`: refresh the short-lived scoped context and inspect the `autoscale-storage` namespace.

## Coding Style & Naming Conventions

Use `gofmt` for Go formatting, idiomatic package names, and descriptive exported identifiers. Give every receiver, parameter, local variable and test variable a meaningful name; do not use single-letter names. Prefer lower camel case matching the type or role: `documentService`, `documentCommand`, `documentHttpConfig`, `httpRequest`, `responseWriter`, `testCase`, `recordIndex`. Apply the same rule to benchmark and deployment scripts. Name Go tests `*_test.go` with `TestXxx` functions. Use versioned, ordered migration filenames. Keep metric labels bounded; never label metrics with record IDs, raw SQL, or arbitrary URLs.

Keep component Config types and metrics with their consuming adapters. Use descriptive names such as `DocumentPostgresConfig` and `DocumentPostgresRepository`. Technical helpers belong in each application’s `internal/utils`. Keep applications as independent Go modules without cross-application imports or local replacements. Prefer native YAML configuration and small explicit duplication over generators or infrastructure schemas in Go. Keep duplicated addresses consistent manually.

In Scala 3, use Scala 3 syntax and derivation throughout; do not declare Scala 2-style `implicit` values. Derive every type class supported by a data type directly (for example, `derives JsonCodec, Loggable, CanEqual, Schema`) and keep only behavior or mappings in companion objects. Apply `@jsonMemberNames(SnakeCase)` to DTOs whose complete wire contract uses snake_case; reserve field-level overrides for exceptional external contracts. Tapir endpoints must use concrete typed `jsonBody[Dto]` inputs and outputs; do not expose application JSON contracts as `stringJsonBody` or serialize them manually in handlers.

Model dependencies through ZIO environment layers. Every service or adapter class with constructor dependencies has a companion `val layer` that lists only those dependencies and uses `ZLayer.derive[...]` whenever derivation can construct it. Also expose a `val module` that composes the service with its concrete dependency modules into a self-contained layer. Build a closed module with `ZLayer.make[...]`; use `ZLayer.makeSome[...]` only when the result intentionally retains external environment requirements, and do not assemble modules with manual `>>>` chains. Keep ordinary service constructors public unless construction itself enforces a real invariant; do not add a public effectful `make` solely to compensate for a private constructor. Keep implementation selection and the final layer graph in the outer application entrypoint; application code must not instantiate dependency-bearing services with `new` or manually pass implementations through a constructor chain.

## Testing Guidelines

Use Go's standard testing package; no coverage threshold is established. PostgreSQL tests use the application YAML and `POSTGRES_PASSWORD`; use `docker compose --profile test run --build --rm tests`. Test routing, concurrent writes, cancellation, and database failures. Record benchmark settings, limits, warm-up, throughput, errors, and latency. Measure pool acquisition separately from database execution. Never report unexecuted benchmarks as results.

## Commit & Pull Request Guidelines

Use `epic/<number>-<short-name>` branches. Commit messages include feature number and name, e.g. `docs: Feature 0.1 — Formal system model`. Leave review changes uncommitted until the user explicitly requests a commit. Explain the implementation, answer questions, and address requested changes. Merge into `master` only after checks pass and the user explicitly approves the merge following review. Never infer merge approval from permission to implement or commit. PRs describe behavior, validation, limitations, and dashboard screenshots. Never commit real credentials.

## Local Execution & Design Decisions

Before starting either stack, ask the user to enable WSL2. For Kubernetes work use the existing k3s cluster and the WSL alias `k3s-kubectl`; do not create kind or manage the k3s service lifecycle unless explicitly requested. Keep project resources in namespace/context `autoscale-storage`, use scoped access for routine operations and never switch the user's current context. Windows does not wildcard-resolve the local Ingress hostname; external local clients must map `api.autoscale-storage.localhost` to the current Traefik/WSL address without changing the Ingress contract. Calibrate explicit resource limits experimentally; smaller limits alone do not ensure representative results. Verify generator and monitoring headroom. Scala DSL deployment remains an open decision.

## Control Plane Observability

The namespace-scoped local operator may use `pods/exec` only for operational artifact export such as `kubectl cp`; runtime application ServiceAccounts remain tokenless.

The Scala Control Plane owns assignment administration and the advisory controller. Keep `assignment`, `systemstate`, and `scaling` as separate feature slices. Prometheus JSON and PromQL belong only to the `systemstate` Prometheus client; HTTP DTOs remain under each feature's `transport/dto`. The active assignment is the authoritative node set for every observation. Recheck its epoch before publishing a collected state so a topology change cannot produce a recommendation for a mixed snapshot.

Scaling policy is a pure transition over `SystemState`, explicit `PolicyState`, configuration, and evaluation time. Missing or stale required metrics produce `InsufficientData`. Advisory decisions never publish assignment or change replicas. `storage_active_requests` is in-flight application work, not queue size or exact admission occupancy. Keep `429` overload separate from `5xx` server errors, and do not turn client `4xx` responses into a scaling signal.

The Control Plane overview dashboard visualizes controller-published aggregates and thresholds, not an independent approximation of policy in Grafana. Missing data must not render as zero. Keep controller metric labels bounded to fixed signal, status, direction, and decision vocabularies. Generated run artifacts belong under ignored `results/`; only selected validation screenshots referenced by epic documentation belong under `docs/epics/epic-4/` after acceptance.

Normalize environment-specific resource telemetry inside the `systemstate` Prometheus adapter. `PrometheusQuerySet` is the common signal-query contract; Docker and Kubernetes implementations must return the same domain `SignalName` set and normalize samples to the `node` label. Preserve Docker as the default profile for CLI compatibility and select Kubernetes explicitly at the deployment entrypoint. Control Plane policy and services must not branch on deployment environment or consume container, Pod, namespace, Deployment, StatefulSet, Kubernetes client or Kubernetes API types.

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
