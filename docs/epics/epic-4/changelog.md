# Эпик 4: планируемый change set и продолжение

## Точка старта

Работу начинать от завершённого состояния [эпика 3](../epic-3/changelog.md). На 11.10.2026 проверены Scala Control Plane, PostgreSQL assignment repository, Router refresh, atomic fencing и полный Compose CRUD smoke. Рабочее дерево эпика 3 может оставаться незакоммиченным: перед изменениями нужно прочитать `git status` и сохранить весь существующий diff.

Текущий Control Plane предоставляет только assignment administration и operational endpoints. Он не читает Prometheus, не строит `SystemState`, не принимает scaling decisions и не управляет infrastructure.

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

Следующий scope разбивается на два самостоятельных инкремента:

1. Kubernetes baseline: Deployment/Service, probes, requests/limits, static replicas и CPU/HPA comparison.
2. Actuation/reconciliation: `ScalingDecision → DesiredClusterState → replicas ready → allocator → assignment publication`, отдельно проработав безопасный scale-in.

Allocator из `benchmarks/partitioning` не импортируется в Scala. Его инварианты и канонические переходы используются как cross-language golden tests для production-реализации fixed logical partitions.

Merge и commit выполняются только после review и отдельного явного разрешения пользователя.
