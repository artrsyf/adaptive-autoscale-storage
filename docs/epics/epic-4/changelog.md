# Эпик 4: change set и продолжение

## Точка старта

Работу начинать от завершённого состояния [эпика 3](../epic-3/changelog.md). На 11.10.2026 проверены Scala Control Plane, PostgreSQL assignment repository, Router refresh, atomic fencing и полный Compose CRUD smoke. Рабочее дерево эпика 3 может оставаться незакоммиченным: перед изменениями нужно прочитать `git status` и сохранить весь существующий diff.

Текущий Control Plane предоставляет только assignment administration и operational endpoints. Он не читает Prometheus, не строит `SystemState`, не принимает scaling decisions и не управляет infrastructure.

## Реализованный change set

В ветке `epic/4-observable-control-plane` реализован завершённый advisory-срез доменной модели и runtime. Добавлены явные единицы сигналов, `MetricWindow`, `DataFreshness`, `ProcessingUnitState`, `DatabaseState`, `ClusterState`, `DataQuality`, `SystemState`, чистый переход policy с явным `PolicyState`, Prometheus adapter, epoch-safe агрегатор, последовательный controller fiber, read-only API и controller metrics. Строгий YAML-конфиг расширен секциями `metrics` и `scaling`.

Scala DTO используют нативный для Scala 3 вывод type class через `derives JsonCodec, Loggable, CanEqual, Schema`; единый snake_case-контракт задаётся `@jsonMemberNames(SnakeCase)`, без field-level аннотаций и ручных `implicit val`. Tapir endpoints описывают конкретные DTO через `jsonBody[Dto]`. Внешняя Prometheus response model выводит только необходимые decoder/equality instances и сохраняет имена upstream API.

Composition root переведён на явный ZLayer-граф: dependency-bearing services, adapters, handlers и routes объявляют `layer` с непосредственными требованиями и самодостаточный `module`; конкретные реализации окончательно выбираются только в `Main`. Для обычных constructor-based компонентов используется `ZLayer.derive`, effectful state (`Ref`) и scoped resources остаются в явных layer constructors. Самодостаточные module собираются через `ZLayer.make`, а `ZLayer.makeSome` оставлен только для графов с намеренно незакрытым environment; ручные `>>>`-цепочки удалены. Конструкторы `AssignmentService` и `ControllerService` открыты, а избыточные публичные effectful-фабрики удалены.

В README добавлено единое бизнесово-техническое описание decision loop: authoritative assignment, источники метрик, max-per-node aggregation, фактические lower/upper thresholds, правила `ScaleOut`/`ScaleIn`/`Maintain`/`InsufficientData`, confirmation/cooldown и advisory-семантика решения, атомарно опубликованного через in-memory `Ref`.

Добавлены provisioned dashboard `Control Plane / System Overview` и размеченный workload `controller`. Фактические PromQL/cAdvisor labels, контейнерная интеграция, thresholds, диагностические нагрузки и визуальная проверка подтверждены в [validation report](validation.md). Dashboard использует instant queries для текущих overview-значений и range queries для timeline; сохранённые скриншоты проверены на причинную согласованность с архивом Prometheus.

## Дополнительная проверка scale-out triggers — 12.10.2026

После основной приёмки закрыт риск, что end-to-end workload проверял только RPS. В `ScalingPolicySpec` добавлен табличный тест, который по очереди делает единственным upper breach каждый из девяти `SignalName.policySignals` и требует `ScaleOut` с соответствующей причиной.

На работающем Compose-стенде проведены три раздельных диагностических прогона:

- CPU pressure внутри cgroup `processing-unit-1`: CPU pressure ratio `1.251`, остальные сигналы ниже `0.466`, опубликован `ScaleOut`;
- memory pressure внутри того же application container с временным limit `80 MiB`: memory pressure ratio `1.028`, остальные сигналы ниже `0.116`, опубликован `ScaleOut`; после прогона limit восстановлен до `192 MiB` и test-data удалены;
- PostgreSQL table lock с конкурентным load-контейнером: thresholds пересекли HTTP p99, active requests, pool acquisition, DB operation, `5xx`, `429` и RPS; опубликован `ScaleOut`, CPU/memory остались ниже upper thresholds.

Отдельный одноразовый Control Plane контейнер проверил отказ Prometheus: `/readyz` остался `200`, state перешёл в `degraded`, source — в `unavailable`, decision — в `InsufficientData`. Контейнер после проверки удалён. Полные команды, timestamps, архивы и значения приведены в [validation report](validation.md).

После изменения повторно прошли `make check`, `make integration` и `make smoke`. Scala suite содержит `32` теста. Compose-стек оставлен запущенным; Docker Engine и WSL не перезапускались. Все изменения остаются незакоммиченными до отдельного указания пользователя.

## Обязательный change set

### Feature 4.1. Domain-модель наблюдаемого состояния

- добавить immutable `SystemState`, `ProcessingUnitState`, cluster aggregates и `DataQuality`;
- определить явные типы/валидацию для ratios, rates, durations и timestamps;
- считать active assignment авторитетным списком нод и сохранять его epoch в состоянии;
- отделить domain types от Prometheus response и HTTP DTO.

### Feature 4.2. Prometheus adapter

- определить узкий `MetricsSource` рядом с consuming service;
- реализовать клиент Prometheus HTTP API на ZIO/zio-http и строгий decode через zio-json;
- добавить timeout, классификацию transport/protocol/data errors и тесты response fixtures;
- подтвердить реальные cAdvisor labels в запущенном стенде до фиксации CPU/RAM PromQL;
- не использовать Grafana как runtime dependency.

### Feature 4.3. State Aggregator

- добавить последовательный polling fiber под lifecycle приложения;
- собирать метрики по нодам текущего assignment;
- вычислять cluster aggregates, missing/stale signals и source health;
- публиковать snapshot атомарно через ZIO `Ref`;
- не блокировать assignment API/readiness при отказе Prometheus;
- запретить использование устаревшего snapshot как нового наблюдения.

### Feature 4.4. Advisory Scaling Policy

- реализовать `ScaleOut`, `ScaleIn`, `Maintain`, `InsufficientData`;
- добавить верхние/нижние thresholds, confirmation windows, cooldown и replica bounds;
- возвращать reasons/blockedBy для каждого решения;
- сохранить policy чистой и детерминированной;
- не подключать решение к assignment publication или replica actuator.

### Feature 4.5. API и метрики

- добавить Tapir endpoints `GET /control/state` и `GET /control/decision`;
- добавить operation-specific zio-json DTO;
- расширить `/metrics` bounded-cardinality показателями polling/data quality/decision;
- сохранить существующие `/livez`, `/readyz`, `/metrics` и Assignment API совместимыми.

### Feature 4.6. Конфигурация, Compose и документация

- расширить строгий `control-plane/config.yaml` секциями `metrics` и `scaling`;
- добавить Prometheus dependency только на уровне сети/конфигурации, без PostgreSQL-связности с Router;
- обновить архитектурную диаграмму и описание деградации;
- создать `validation.md` только после реальных проверок, с командами, датой и результатами;
- не записывать планируемые эксперименты как выполненные.

## Рекомендуемая структура Scala-кода

Сохранить feature slices и потребительские порты. Один из допустимых вариантов:

```text
control-plane/src/main/scala/storage/controlplane/
├── systemstate/
│   ├── domain/
│   ├── service/
│   ├── client/prometheus/
│   └── transport/{dto,http}/
├── scaling/
│   ├── domain/
│   ├── service/
│   └── transport/{dto,http}/
├── assignment/                       # существующий модуль, не смешивать
├── app/
└── config/
```

Не создавать generic root `client`, `service` или универсальный HTTP handler. Если state и decision отдаются одним controller endpoint group, граница должна оставаться явной в доменной модели и DTO.

## Порядок реализации

1. Прочитать документы эпиков 2–4, актуальную архитектуру и код Control Plane.
2. Зафиксировать domain types, обязательные сигналы, единицы и правила `DataQuality`.
3. Реализовать и протестировать pure policy до фонового runtime.
4. Реализовать Prometheus response model/client и unit fixtures.
5. Собрать `StateAggregator` и проверить его на fake `MetricsSource` с ZIO TestClock.
6. Подключить fiber в composition root и добавить HTTP DTO/routes.
7. Добавить config, controller metrics и Compose connectivity.
8. Выполнить статические, unit, container integration и smoke проверки.
9. Провести только короткие diagnostic workloads; сравнительный benchmark оставить для B6.
10. Обновить `README`, `architecture`, roadmap и фактический validation report.

## Обязательные тестовые сценарии

- полный набор свежих series для всех active nodes;
- отсутствующий обязательный signal у одной ноды;
- stale sample и полностью недоступный Prometheus;
- лишняя series от ноды, отсутствующей в active assignment;
- смена assignment epoch между двумя polling cycles;
- timeout одного запроса без остановки последующих циклов;
- отсутствие overlap при цикле дольше обычного poll duration;
- upper threshold один и несколько окон подряд;
- scale-in только при выполнении всех lower thresholds;
- hysteresis band, cooldown, min/max bounds;
- рестарт Control Plane и `InsufficientData` до накопления окон;
- регрессия Assignment API, Router refresh, fencing и CRUD.

## Команды проверки

Без контейнеров:

```powershell
make check
Push-Location control-plane; sbt -batch test; Pop-Location
docker compose config --quiet
git diff --check
```

Контейнерные проверки после подтверждения пользователем доступности WSL2:

```powershell
docker compose --profile test run --build --rm control-plane-tests
make test-all
```

Diagnostic workload запускается отдельно от acceptance suite, с сохранением фактического config и указанием, что решение работало в advisory-режиме.

## Условия остановки scope

Эпик завершён, когда Control Plane стабильно показывает объяснимое неисполняемое решение на основании свежего `SystemState`, а отказ Prometheus безопасно приводит к `InsufficientData` без регрессии assignment/fencing.

В рамках этого change set не следует:

- добавлять Kubernetes manifests или HPA;
- вызывать API Docker/Kubernetes;
- автоматически публиковать новый assignment;
- переносить allocator и объявлять topology reconciliation реализованным;
- вводить Kafka/Redis или физическое перемещение данных.

## Следующая точка после завершения

Следующий scope зафиксирован в [эпике 5](../epic-5/README.md): воспроизводимый Kubernetes static baseline, стабильная identity Processing Units, placement contract, probes, requests/limits и Kubernetes-наблюдаемость. CPU/HPA comparison исключён из этого инкремента, потому что replica count ещё не связан с authoritative assignment.

После него отдельный actuation/reconciliation scope должен реализовать `ScalingDecision → DesiredClusterState → replicas ready → allocator → assignment publication`, отдельно проработав membership и безопасный scale-in.

Allocator из `benchmarks/partitioning` не импортируется в Scala. Его инварианты и канонические переходы используются как cross-language golden tests для production-реализации fixed logical partitions.

Merge и commit выполняются только после review и отдельного явного разрешения пользователя.
