# Эпик 4: наблюдающий Control Plane v1

## Статус

Эпик реализован в ветке `epic/4-observable-control-plane` после завершённого runtime-среза [эпика 3](../epic-3/README.md). Он использует существующие метрики и Scala 3/ZIO Control Plane, но не изменяет число processing unit и не публикует assignment автоматически. Контейнерная, нагрузочная и визуальная приёмка выполнена 11.10.2026; 12.10.2026 отдельными контейнерными сценариями дополнительно подтверждены все policy-trigger и безопасная деградация при недоступном Prometheus. Команды, результаты и причинный разбор приведены в [validation report](validation.md).

Фактический change set и continuation notes: [changelog](changelog.md).

## Цель и место в roadmap

Эпик реализует B3 `Control Plane v1` и чистое decision-ядро для последующего B5:

```text
Prometheus
    ↓
MetricsSource
    ↓
StateAggregator
    ↓
SystemState
    ↓
ScalingPolicy
    ↓
ScaleOut / ScaleIn / Maintain / InsufficientData
```

Решение работает в advisory-режиме: его можно наблюдать через HTTP и метрики, но оно не является командой на изменение инфраструктуры. Такое разделение позволяет проверить семантику сигналов, полноту состояния, hysteresis и cooldown до появления Kubernetes-адаптера и reconciliation loop.

Эпик опирается на уже реализованные части:

- Prometheus собирает метрики Router, processing unit, PostgreSQL, контейнеров и нагрузочного генератора;
- Grafana содержит запросы для RPS, latency, active requests, pool acquisition и DB execution;
- Control Plane уже имеет ZIO lifecycle, Tapir/zio-http transport, zio-json, строгий YAML-конфиг и доступ к активному assignment;
- active assignment задаёт авторитетный состав processing unit и текущий epoch;
- существующие `constant`, `ramp` и `burst` workload подходят для диагностической проверки решений.

## Границы

В scope входят:

- чтение instant/range query из Prometheus через узкий порт `MetricsSource`;
- типизированная модель наблюдаемого состояния системы;
- периодическая агрегация без пересекающихся циклов опроса;
- оценка полноты и свежести данных;
- детерминированная multi-metric policy с защитой от колебаний;
- read-only API текущего состояния и последней рекомендации;
- метрики самого control loop;
- unit и контейнерная интеграционная проверка.

Вне scope остаются:

- изменение replica count, Kubernetes API и HPA;
- автоматический вызов `PUT /assignments/current`;
- membership/service discovery вне уже активного assignment;
- перенос allocator из исследовательского Go harness в runtime;
- physical migration, draining и partition state machine;
- Kafka, Redis, predictive scaling и ML;
- долговременное хранение истории наблюдений и решений.

Последнее успешно вычисленное состояние можно хранить в памяти Control Plane. После рестарта решение остаётся `InsufficientData`, пока не накоплено необходимое число валидных окон.

## Источники и семантика сигналов

Первая версия использует метрики, которые уже экспортируются стендом:

| Сигнал | Источник | Назначение |
|---|---|---|
| RPS | `rate(storage_requests_total{role="processing-unit"}[window])` | интенсивность фактически обработанного потока |
| error/overload rate | `storage_requests_total` по status, включая `429` | ошибки и насыщение admission |
| p99 HTTP latency | `storage_request_duration_seconds_bucket` | пользовательская задержка processing unit |
| active requests | `storage_active_requests` | текущая конкуренция внутри processing unit |
| pool acquisition p95 | `storage_pool_acquire_seconds_bucket` | насыщение пула соединений отдельно от БД |
| DB operation p95 | `storage_db_operation_seconds_bucket` | сеть, locks, SQL и commit после получения соединения |
| CPU/RAM | cAdvisor | загрузка контейнеров processing unit |
| partition skew | `storage_partition_requests_total` | наблюдаемый диагностический сигнал, но не trigger первой policy |

`queue_size` не моделируется через `active_requests`: отдельной очереди в текущем processing unit нет. Если очередь появится позднее, она должна получить собственный bounded metric и отдельную семантику.

PromQL-запросы принадлежат Prometheus-адаптеру или его конфигурации. Доменная модель не содержит Prometheus metric names, labels или JSON API types. Перед фиксацией CPU/RAM query контейнерная проверка должна подтвердить фактические cAdvisor labels, а не полагаться на имя контейнера по памяти.

## Как Control Plane оценивает систему

Бизнесовая задача текущего Control Plane — ответить, достаточно ли кластеру имеющейся вычислительной ёмкости и стоит ли рекомендовать добавить или убрать одну processing unit. Это наблюдающий советник: он не запускает контейнеры, не вызывает Kubernetes или Docker API, не меняет assignment и не перемещает partitions.

Каждый цикл начинается с чтения authoritative assignment из PostgreSQL. Assignment задаёт epoch и точный список processing unit, которые должны присутствовать в наблюдении. После этого Control Plane запрашивает Prometheus, строит состояние по каждой ожидаемой ноде и повторно читает assignment. Если за время сбора изменился epoch или состав нод, смешанный snapshot отбрасывается.

Prometheus объединяет данные из трёх эксплуатационных контуров:

- application-метрики processing unit дают фактический поток запросов, HTTP latency, статусы ответов и число исполняющихся запросов;
- PostgreSQL adapter каждой processing unit отдельно измеряет ожидание соединения из pool и выполнение операции после получения соединения;
- cAdvisor измеряет CPU и память контейнеров относительно их cgroup limits.

Для policy используется наиболее неблагополучная processing unit: по каждому сигналу берётся максимум среди всех нод активного assignment. Такой подход не позволяет среднему значению скрыть hot partition или локальную деградацию одной ноды. Суммарный RPS кластера рассчитывается отдельно для наблюдаемости, но scaling policy оценивает максимальный per-node RPS.

### Пороговая модель

У каждого policy-сигнала есть два порога. Верхний означает давление, при котором требуется рассмотреть scale-out. Нижний означает достаточно свободное состояние для рассмотрения scale-in. Пространство между ними является hysteresis band и приводит к `Maintain`.

| Сигнал | Lower | Upper | Эксплуатационный смысл |
|---|---:|---:|---|
| RPS наиболее загруженной processing unit | `20 req/s` | `80 req/s` | фактически обработанный поток на одну ноду |
| HTTP p99 | `50ms` | `200ms` | пользовательская задержка в хвосте распределения |
| server error rate | `0.1%` | `2%` | только ответы `5xx`; клиентские `4xx` не являются scaling-сигналом |
| overload rate | `0.1%` | `1%` | доля `429`, показывающая насыщение admission |
| active requests | `8` | `48` | исполняющаяся работа; не queue size |
| pool acquisition p95 | `10ms` | `100ms` | ожидание свободного PostgreSQL connection |
| DB operation p95 | `50ms` | `250ms` | сеть, locks, SQL и commit после получения connection |
| CPU utilization | `35%` | `80%` | доля cgroup CPU quota processing unit |
| memory utilization | `40%` | `85%` | доля cgroup memory limit processing unit |

HTTP p95 также собирается и отображается, но не входит в policy v1. Для scaling используется p99, потому что средние значения и p95 могут оставаться нормальными при уже заметной деградации наиболее медленных запросов. `storage_active_requests` не называется очередью: отдельной очереди в runtime нет, и подмена её размера числом активных запросов дала бы ложную семантику.

Текущие thresholds являются диагностической конфигурацией локального стенда, а не production SLO. Они выбраны так, чтобы нормальный режим имел заметный запас, а каждый trigger можно было воспроизводимо пересечь отдельным нагрузочным сценарием. CPU `80%` и memory `85%` оставляют запас под bursts и runtime overhead; active requests `48` составляют 75% от admission limit `64`; `429` при `1%` означает уже наблюдаемое отбрасывание работы; DB p95 `250ms` позволяет реагировать существенно раньше statement timeout `2.5s`. Lower thresholds намеренно заметно ниже upper: возврата из перегрузки недостаточно для scale-in, система должна быть устойчиво свободной.

Перед production пороги должны быть повторно откалиброваны по SLO, измеренной ёмкости одной реплики, распределению ключей, времени запуска реплики, стоимости инфраструктуры и доступной ёмкости PostgreSQL. Фактическая проверка текущих значений приведена в [validation report](validation.md).

### Формирование вердикта

Policy сначала проверяет качество данных. Для каждой активной ноды должны присутствовать свежие и однозначные series всех обязательных сигналов. Недоступный Prometheus, missing или stale series, duplicate series, неизвестная нода либо изменение assignment во время сбора дают `InsufficientData`. Monitoring failure не делает Control Plane unready и не останавливает assignment API, но запрещает actionable recommendation.

При полном состоянии применяются следующие правила:

- `ScaleOut` становится кандидатом, если хотя бы один обязательный сигнал достиг upper threshold; логика между сигналами — OR;
- `ScaleIn` становится кандидатом, только если все обязательные сигналы одновременно не выше lower thresholds; логика — AND;
- `Maintain` означает hysteresis band, незавершённое подтверждение, активный cooldown либо достижение replica bounds;
- `InsufficientData` означает, что безопасное решение по текущим данным невозможно.

Кандидат должен повториться в трёх последовательных evaluation. Polling выполняется каждые `5s`, а rate и percentile сигналы уже рассчитаны по rolling window `30s`, поэтому краткий spike не превращается в немедленную рекомендацию. После подтверждённой рекомендации действует cooldown `30s`. Изменение ограничено одним шагом, `minReplicas=1`, `maxReplicas=10`.

Решение объяснимо: вместе с направлением публикуются фактические значения, пересечённые thresholds, текущий и рекомендуемый replica count, assignment epoch, время оценки и блокирующие условия. Если одновременно нарушены HTTP p99, pool wait и error rate, в reasons присутствуют все три причины.

### Что означает ScaleOut или ScaleIn

`ScaleOut` и `ScaleIn` — immutable advisory-значения, а не команды инфраструктуре. Например, `currentReplicas=3` и `recommendedReplicas=4` означают только рекомендацию следующему actuator/reconciliation increment. Текущее число реплик определяется количеством нод в authoritative assignment; Control Plane не сверяется с Docker или Kubernetes и не утверждает, что рекомендуемая реплика уже создана.

Последний controller snapshot атомарно хранится в ZIO `Ref`. В snapshot входят `SystemState`, решение, confirmation state, cooldown и служебные счётчики циклов. `Ref` обеспечивает согласованное чтение через HTTP и `/metrics`, но не превращает решение в mutable action. Это in-memory состояние: после рестарта confirmation и cooldown начинаются заново, Control Plane переходит в `collecting`/`InsufficientData` и обязан снова накопить свежие подтверждённые окна. История опубликованных метрик остаётся в Prometheus, но Prometheus не является хранилищем policy state.

## Доменная модель

Минимальная модель состояния:

```text
SystemState
├── observedAt
├── window
├── assignmentEpoch
├── processingUnits[]
│   ├── nodeId
│   ├── rps
│   ├── errorRate
│   ├── p99Latency
│   ├── activeRequests
│   ├── poolAcquireP95
│   ├── databaseOperationP95
│   ├── cpuUtilization
│   └── memoryUtilization
├── cluster aggregates
└── DataQuality
    ├── complete
    ├── missingSignals[]
    ├── staleSignals[]
    └── sourceError
```

Авторитетный набор `nodeId` берётся из текущего assignment. Неожиданные series не должны молча становиться членами topology, а отсутствие series для активной ноды должно отражаться в `DataQuality`.

Значения хранят единицы измерения явно: duration, ratio или rate не должны смешиваться как безразмерные `Double`. Wire DTO отделены от domain types.

## Агрегация и lifecycle

`StateAggregator` работает как управляемый ZIO fiber:

1. читает текущий assignment и фиксирует epoch/набор нод для цикла;
2. выполняет ограниченные timeout запросы к Prometheus;
3. сопоставляет series только с нодами этого assignment;
4. проверяет timestamp каждого результата и обязательные сигналы;
5. атомарно заменяет опубликованный `SystemState`;
6. передаёт валидное состояние policy или публикует `InsufficientData`;
7. ждёт `poll_interval` после завершения цикла, не допуская overlap.

Ошибка одного цикла не завершает приложение. Используется ограниченный backoff, а устаревший успешный snapshot не выдаётся как свежий. Остановка приложения прерывает fiber и ожидает его завершения в пределах общего shutdown timeout.

Prometheus является источником наблюдения, а не зависимостью assignment safety. Его недоступность:

- не меняет `/livez`;
- не делает недоступными assignment endpoints;
- не должна ломать текущую PostgreSQL readiness-проверку;
- переводит controller status в degraded/insufficient-data;
- запрещает выдавать `ScaleOut` или `ScaleIn` по старым данным.

## Scaling policy v1

Policy является чистой функцией доменных входов и собственного явно переданного состояния стабилизации. Результат содержит:

```text
ScalingDecision
├── kind: ScaleOut | ScaleIn | Maintain | InsufficientData
├── currentReplicas
├── recommendedReplicas
├── evaluatedAt
├── assignmentEpoch
├── reasons[]
└── blockedBy[]
```

Начальная policy должна обеспечивать:

- `InsufficientData` при missing/stale обязательных сигналах;
- scale-out при устойчивом нарушении хотя бы одного верхнего safety threshold;
- scale-in только при одновременном нахождении всех обязательных сигналов ниже нижних thresholds;
- `Maintain` в hysteresis band;
- подтверждение решения несколькими последовательными окнами;
- cooldown после подтверждённой рекомендации;
- ограничения `min_replicas`, `max_replicas` и шага изменения;
- объяснимые причины с именами доменных сигналов, но без raw PromQL;
- одинаковый результат для одинакового состояния и одинаковой истории policy.

Пороговые значения являются конфигурацией эксперимента, а не зашитыми константами. Их начальные значения должны быть документированы как диагностические и позднее откалиброваны отдельными workload-прогонами.

## Конфигурация

`control-plane/config.yaml` расширяется двумя строгими секциями с проверкой неизвестных ключей:

```yaml
metrics:
  prometheus_url: http://prometheus:9090
  poll_interval: 5s
  query_timeout: 2s
  observation_window: 30s
  max_sample_age: 15s

scaling:
  min_replicas: 1
  max_replicas: 10
  step: 1
  confirmation_windows: 3
  cooldown: 30s
  # Отдельные верхние и нижние thresholds для используемых сигналов.
```

Точные названия threshold-полей фиксируются вместе с domain policy, чтобы конфигурация не дублировала PromQL и не вводила неиспользуемые параметры. Все duration и диапазоны проверяются на старте.

## HTTP и наблюдаемость контроллера

Через Tapir добавляются read-only endpoints:

```http
GET /control/state
GET /control/decision
```

Ответ состояния содержит агрегированные значения, timestamp, assignment epoch и качество данных. Ответ решения содержит последнюю рекомендацию, причины и ограничения. До первого успешного окна endpoints возвращают корректное представление `collecting`/`insufficient_data`, а не фиктивные нули.

`/metrics` Control Plane дополняется bounded-cardinality метриками:

- число успешных и неуспешных циклов по ограниченному outcome;
- длительность цикла;
- возраст последнего полного состояния;
- текущий тип решения;
- количество переходов решений;
- признак degraded состояния.

Идентификаторы запросов, raw PromQL и тексты ошибок не используются как labels.

## High-level Grafana dashboard

Отдельный provisioned dashboard `Control Plane / System Overview` показывает controller status, advisory decision, качество и возраст данных, current/recommended replicas, confirmation и cooldown. Временные графики используют опубликованные Control Plane значения и thresholds, то есть не реализуют вторую независимую policy в Grafana. Отдельно отображаются workload stage, controller/decision timelines, RPS, p95/p99, error/overload rate, CPU/RAM, active requests, pool и DB latency.

Значения каждого policy signal дополнительно нормализуются как `value / upper threshold` для общего bar gauge. Отсутствующий сигнал не подменяется нулём. Визуальная приёмка выполняется на размеченном workload `controller` с фазами low, steady, ramp, short burst, sustained high и recovery. Снимки и разбор интервалов добавляются только после фактического прогона.

## Критерии готовности

- domain/service код не зависит от Prometheus JSON, HTTP, Tapir или PostgreSQL;
- Prometheus client строго декодирует success/error responses и корректно обрабатывает пустые vectors;
- active assignment определяет ожидаемый состав нод и epoch каждого `SystemState`;
- missing, stale, duplicate и unknown-node series имеют однозначное поведение и тесты;
- циклы polling не пересекаются, переживают временную ошибку и корректно завершаются;
- policy покрыта табличными тестами для scale-out, scale-in, hysteresis, confirmation, cooldown и bounds;
- недоступный Prometheus никогда не приводит к actionable scaling recommendation;
- существующие assignment API, fencing и CRUD smoke остаются неизменными;
- `make check`, контейнерные Control Plane integration tests и `make test-all` проходят;
- диагностические `constant`, `ramp` и `burst` прогоны демонстрируют наблюдаемое изменение recommendation без actuation;
- отдельный `controller` workload размечает фазы нагрузки, а новый dashboard проверен скриншотами всех панелей и причинным разбором каждого интервала;
- документация фиксирует фактические PromQL, config и validation evidence, не выдавая диагностический прогон за сравнительный эксперимент.

## Путь после эпика

Следующий runtime-инкремент должен добавить Kubernetes baseline и только затем actuator/reconciliation:

```text
ScalingDecision
    ↓
DesiredClusterState
    ↓
Kubernetes replica change
    ↓
ожидание Ready processing units
    ↓
fixed-logical-partition allocator
    ↓
публикация assignment epoch
```

Для scale-in порядок будет отдельным протоколом: прекратить новое назначение, выполнить draining и только после безопасного перехода уменьшить replicas. Общая PostgreSQL устраняет физический перенос строк в текущем стенде, но не устраняет in-flight requests и требования fencing.
