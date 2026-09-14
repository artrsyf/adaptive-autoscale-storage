# План выполнения ВКРМ

## Тема

**Распределённая система хранения и обработки данных с адаптивным масштабированием вычислительных узлов**

---

# 1. Логика прохождения плана

План строится не вокруг изучения отдельных технологий, а вокруг последовательного усложнения одной распределённой системы.

Общий маршрут:

1. Формальная модель системы.
2. Один вычислительный узел и PostgreSQL.
3. Логические разделы данных.
4. Несколько вычислительных узлов и маршрутизация.
5. Версионирование топологии.
6. Kafka и идемпотентная обработка.
7. Кэширование.
8. Наблюдаемость.
9. Control Plane.
10. Kubernetes и базовый autoscaling.
11. Классификация состояния и bottleneck detection.
12. Rebalancing и безопасная миграция разделов.
13. Отказоустойчивость.
14. Hot partitions и load-aware rebalancing.
15. Adaptive Controller.
16. Reconciliation loop.
17. Benchmark framework.
18. RQ1/RQ2/RQ3 и fault experiments.

Принцип прохождения каждого эпика:

> **теория → формальная модель → реализация → тесты → метрики → выводы**

Следующий эпик не должен скрывать нерешённые проблемы предыдущего.

---

# 2. Entry point — формализация системы

## Цель

До основной реализации определить сущности системы, границы ответственности компонентов, инварианты и сценарии изменения топологии.

## Feature 0.1. Формальная модель системы

```text
                   Client / API
                       |
                       v
                 +-----------+
                 |  Router   |
                 +-----+-----+
                       |
          +------------+------------+
          |            |            |
          v            v            v
        +----+       +----+       +----+
        | N1 |       | N2 |       | N3 |
        +--+-+       +--+-+       +--+-+
           |            |            |
           +------------+------------+
                        |
             +----------+----------+
             |                     |
             v                     v
          Kafka                  Redis
             |                     |
             +----------+----------+
                        |
                        v
                   PostgreSQL

                    ^
                    |
             +------+------+
             | Control     |
             | Plane       |
             +-------------+
```

### Теория

- распределённая система;
- node / cluster;
- data plane / control plane;
- stateful / stateless components;
- horizontal / vertical scaling;
- partitioning;
- replication;
- routing;
- load balancing;
- shared-nothing architecture;
- Space-Based Architecture;
- data locality;
- separation of compute and storage.

Нужно чётко различать:

```text
partitioning ≠ replication
routing ≠ load balancing
scaling ≠ rebalancing
compute node ≠ database shard
Kafka partition ≠ logical data partition
```

## Feature 0.2. Модель данных

```text
Record
{
    id
    partition_key
    payload
    created_at
    version
}
```

Минимальный API:

```text
GET(key)
PUT(key, value)
DELETE(key)
GET_BATCH(keys)
```

## Feature 0.3. Инварианты

1. Каждый активный логический раздел имеет владельца.
2. Операции записи принимает только актуальный владелец.
3. Версия назначения владельца монотонно возрастает.
4. Повторное выполнение операции не нарушает корректность состояния.
5. После завершения перераспределения каждый раздел имеет ровно одного активного владельца.
6. Постоянное состояние не зависит от жизненного цикла вычислительного узла.

## Результат

Отдельный технический документ с моделями данных, узла, логического раздела, ownership, routing, epoch и базовыми сценариями `3 → 5 → 2 nodes`.

---

# 3. Эпик 1. Базовый вычислительный узел

## Цель

```text
Client
   ↓
Node
   ↓
PostgreSQL
```

Без Kafka, Redis, Kubernetes и автомасштабирования.

## Feature 1.1. API вычислительного узла

Пример:

```text
PUT /records/{key}
GET /records/{key}
DELETE /records/{key}
```

### Теория

- REST / gRPC;
- request lifecycle;
- connection pooling;
- timeout;
- context/cancellation;
- backpressure;
- graceful shutdown;
- Go concurrency model;
- goroutine;
- channel;
- mutex;
- worker pool.

## Feature 1.2. PostgreSQL Repository

### Теория

- ACID;
- transactions;
- isolation levels;
- MVCC;
- WAL;
- connection pool;
- prepared statement;
- B-tree;
- primary key;
- optimistic concurrency;
- version field;
- lost update.

Разобрать сценарии: Node crash, DB timeout, duplicate request, concurrent PUT.

## Feature 1.3. Первый нагрузочный тест

Собирать:

- throughput;
- p50/p95/p99;
- error rate;
- CPU;
- RAM;
- PostgreSQL latency.

### Теория

- throughput;
- latency;
- percentiles;
- Little's Law;
- saturation;
- bottleneck;
- queueing;
- coordinated omission;
- warm-up;
- steady state.

## Done

Есть воспроизводимый baseline и графики latency/throughput от offered load.

---

# 4. Эпик 2. Логическое разделение данных

## Feature 2.1. Hash partitioning

```text
partition = hash(key) % P
```

Например `P = 128`.

### Теория

- horizontal partitioning;
- sharding;
- hash partitioning;
- range partitioning;
- consistent hashing;
- rendezvous hashing;
- virtual nodes;
- logical/virtual partitions.

Ключевой переход:

```text
key → logical partition → assignment table → node
```

вместо:

```text
key → hash(key) % nodeCount → node
```

## Feature 2.2. Доменная модель разделов

Сущности:

```text
PartitionId
PartitionCount
Partitioner
```

## Done

Количество логических разделов не зависит от количества вычислительных узлов.

---

# 5. Эпик 3. Несколько вычислительных узлов

## Feature 3.1. Partition Assignment

```text
P0 → N1
P1 → N2
P2 → N3
P3 → N1
...
```

### Теория

- ownership;
- membership;
- cluster topology;
- metadata;
- partition assignment;
- consistent hashing;
- virtual shards;
- rendezvous hashing;
- load balancing algorithms.

## Feature 3.2. Router

```text
key
 ↓
hash
 ↓
partition
 ↓
owner(partition)
 ↓
node
```

### Теория

- client-side routing;
- proxy routing;
- server-side routing;
- service discovery;
- stale metadata;
- topology version;
- consistent routing.

## Done

Запрос всегда направляется владельцу соответствующего логического раздела.

---

# 6. Эпик 4. Версионирование топологии

## Feature 4.1. ClusterEpoch / PartitionEpoch

```text
P7:
owner = Node3
epoch = 42
```

### Теория

- logical clock;
- Lamport clock — концептуально;
- generation number;
- fencing token;
- stale write;
- optimistic concurrency control;
- ABA problem — обзорно;
- monotonic version.

Принцип:

```text
newEpoch > oldEpoch
```

## Done

Устаревшая карта размещения не позволяет выполнить запись от имени старого владельца.

---

# 7. Эпик 5. Kafka и асинхронная обработка

## Feature 5.1. Producer / Consumer

### Теория

- broker;
- topic;
- partition;
- producer;
- consumer;
- consumer group;
- offset;
- commit;
- replication factor;
- leader/follower;
- ISR;
- retention;
- batching;
- ordering;
- at-most-once;
- at-least-once;
- exactly-once semantics.

## Feature 5.2. Idempotency

```text
operation_id = UUID
```

### Теория

- idempotency;
- duplicate delivery;
- idempotent consumer;
- deduplication;
- transactional outbox;
- inbox pattern;
- retry;
- poison message;
- dead-letter queue.

Обязательный сценарий: consumer записал данные в PostgreSQL и упал до фиксации offset.

## Done

Повторная доставка не нарушает состояние данных.

---

# 8. Эпик 6. Кэширование

## Feature 6.1. L1 cache

```text
Node
 ├── L1
 └── PostgreSQL
```

### Теория

- cache-aside;
- read-through;
- write-through;
- write-back;
- TTL;
- eviction;
- LRU/LFU;
- cache invalidation;
- cache coherence;
- stale data.

## Feature 6.2. Redis L2

```text
Node
 ↓
L1
 ↓ miss
Redis L2
 ↓ miss
PostgreSQL
```

### Теория

- distributed/shared cache;
- local vs shared cache;
- serialization;
- network overhead;
- invalidation.

## Feature 6.3. Эксперимент

Сравнить:

```text
PostgreSQL only
L1 + PostgreSQL
L1 + Redis + PostgreSQL
```

Метрики: p95/p99, DB QPS, cache hit ratio, network, memory.

---

# 9. Эпик 7. Наблюдаемость

## Feature 7.1. Метрики узлов

- requests/sec;
- p50/p95/p99;
- CPU;
- memory;
- active requests;
- queue size.

## Feature 7.2. Метрики Kafka

- consumer lag;
- messages/sec;
- produce latency;
- consume latency.

## Feature 7.3. Метрики PostgreSQL

- query latency;
- connections;
- QPS.

## Feature 7.4. Метрики логических разделов

- requests/sec per partition;
- bytes/sec;
- latency;
- owner;
- skew.

### Теория

- observability;
- metrics/logs/traces;
- RED method;
- USE method;
- histogram;
- counter;
- gauge;
- percentile;
- time series;
- Prometheus model;
- sampling/window;
- moving average;
- EWMA.

## Done

По метрикам можно определить текущее состояние кластера и вероятный bottleneck.

---

# 10. Эпик 8. Control Plane v1

## Feature 8.1. SystemState

```scala
case class SystemState(
  cpu: Double,
  memory: Double,
  rps: Double,
  p99: Duration,
  kafkaLag: Long,
  dbLatency: Duration,
  partitionLoads: Map[PartitionId, Double],
  nodeCount: Int
)
```

### Теория

- state-space representation;
- feedback control;
- control loop;
- observable state;
- sampling interval;
- noise;
- moving average;
- hysteresis.

## Feature 8.2. Базовая функция решения

```scala
def decide(state: SystemState): ScalingDecision
```

Сначала:

```text
Maintain
ScaleOut
ScaleIn
```

---

# 11. Эпик 9. Kubernetes

## Feature 9.1. Deployment

### Теория

- Pod;
- Deployment;
- ReplicaSet;
- Service;
- readiness probe;
- liveness probe;
- requests/limits;
- scheduler;
- ConfigMap;
- Secret.

## Feature 9.2. HPA baseline

### Теория

- Horizontal Pod Autoscaler;
- resource metrics;
- target utilization;
- desired replicas;
- stabilization window;
- scale-up/scale-down policy;
- cooldown;
- oscillation/thrashing.

HPA используется как baseline для дальнейшего исследования.

---

# 12. Эпик 10. Control Plane v2 — базовое автомасштабирование

## Feature 10.1. Threshold scaling

```text
CPU > 70% → ScaleOut
CPU < 30% → ScaleIn
```

### Теория

- reactive autoscaling;
- threshold control;
- hysteresis;
- cooldown;
- oscillation;
- delayed feedback;
- stabilization.

## Feature 10.2. Многомерное состояние

Использовать CPU, RPS, p99, Kafka lag, DB latency и partition skew.

### Теория

- bottleneck analysis;
- saturation;
- queueing theory;
- backpressure;
- overload;
- resource contention;
- feedback control;
- multi-variable control;
- reactive vs predictive autoscaling;
- control stability.

---

# 13. Эпик 11. Классификация состояния системы

```text
SystemState
    ↓
classify
    ↓
SystemCondition
    ↓
decide
    ↓
Action
```

Состояния:

```text
Healthy
ComputeBound
QueueBound
DatabaseBound
HotPartition
Overprovisioned
```

### Теория

- rule-based classification;
- threshold classification;
- state machine;
- decision table;
- workload classification;
- fuzzy logic — related work;
- anomaly detection — обзорно.

Примеры:

```text
QueueBound := kafkaLag > L AND dbLatency < DB_MAX
DatabaseBound := dbLatency > DB_MAX
HotPartition := max(partitionRPS) / mean(partitionRPS) > K
```

---

# 14. Эпик 12. Rebalancing

## Feature 12.1. Новый Partition Assignment

### Теория

- repartitioning;
- rebalancing;
- consistent hashing;
- rendezvous hashing;
- virtual shards;
- minimal movement;
- load-aware partition assignment;
- bin packing — обзорно.

## Feature 12.2. Scale-out `3 → 5`

1. Control Plane принимает решение.
2. Desired replicas увеличивается.
3. Kubernetes запускает Pods.
4. Ожидается readiness.
5. Новый узел регистрируется.
6. Рассчитывается новая карта разделов.
7. Выполняется rebalancing.
8. Старые владельцы освобождают разделы.
9. Новая карта активируется.

## Feature 12.3. Scale-in `5 → 2`

1. Выбрать удаляемые узлы.
2. Назначить их разделы другим узлам.
3. Перевести узлы в DRAINING.
4. Не направлять новые операции.
5. Завершить in-flight операции.
6. Подтвердить transfer ownership.
7. Уменьшить replicas.

---

# 15. Эпик 13. Безопасная миграция раздела

## Feature 13.1. Partition State Machine

```text
OWNED(Node1)
      ↓
PREPARING
      ↓
TRANSFERRING(Node1 → Node4)
      ↓
ACTIVATING(Node4)
      ↓
OWNED(Node4)
```

### Теория

- finite-state machine;
- distributed state machine;
- ownership transfer;
- lease;
- fencing token;
- epoch;
- split brain;
- stale owner.

Главный инвариант: операция записи в каждый момент имеет однозначно определённого допустимого владельца.

## Feature 13.2. Graceful draining

### Теория

- graceful shutdown;
- draining;
- in-flight requests;
- quiescence;
- handoff;
- acknowledgement;
- timeout;
- retry.

## Done

`3 → 5` и `5 → 2` выполняются без неопределённого ownership.

---

# 16. Эпик 14. Kafka во время rebalancing

Нужно определить, кто обрабатывает события раздела во время переноса и с какого offset продолжает новый владелец.

### Теория

- consumer group rebalance;
- partition ownership;
- committed offset;
- offset transfer;
- duplicate processing;
- ordering guarantees;
- idempotent processing;
- consumer fencing;
- exactly-once processing;
- transactional consumption.

Инварианты:

```text
No lost operation
Duplicates are tolerated through idempotency
Required ordering is preserved
```

---

# 17. Эпик 15. Cache migration

MVP:

```text
Old owner → invalidate/discard L1
New owner → cold cache → PostgreSQL/Redis → warm-up
```

### Теория

- cache warm-up;
- cold cache;
- invalidation;
- cache coherence;
- cache migration;
- prewarming;
- replication.

Эксперимент: p99 spike после rebalancing и время возврата к steady state.

---

# 18. Эпик 16. Отказы

## Feature 16.1. Node crash

```text
Node dies
 ↓
detect
 ↓
mark unavailable
 ↓
reassign partitions
 ↓
start replacement
 ↓
recover
```

### Теория

- failure model;
- crash-stop;
- crash-recovery;
- partial failure;
- timeout;
- heartbeat;
- failure detector;
- false positive;
- MTTR.

## Feature 16.2. Failure during migration

Рассмотреть падение old owner, new owner и Control Plane.

### Теория

- distributed transaction basics;
- atomicity;
- 2PC — для понимания;
- consensus — conceptual;
- quorum — conceptual;
- fencing;
- idempotency;
- recovery state machine.

Не реализовывать собственный consensus protocol.

---

# 19. Эпик 17. Hot Partition Detection

## Feature 17.1. Skew metric

Пример:

```text
Skew = max(load_i) / mean(load_i)
```

### Теория

- skew;
- load imbalance;
- Zipf distribution;
- coefficient of variation;
- standard deviation;
- max/mean;
- percentiles;
- Gini coefficient — опционально;
- entropy — опционально.

## Feature 17.2. Detector

Определить threshold, временное окно, минимальную длительность перегрева и защиту от false positive.

---

# 20. Эпик 18. Load-aware rebalancing

Варианты действия:

```text
Maintain
Rebalance
ScaleOut
ScaleOut + Rebalance
```

### Теория

- dynamic load balancing;
- load-aware placement;
- scheduling;
- bin packing;
- greedy algorithms;
- migration cost;
- cost function.

Возможная модель:

```text
Cost = α * latency + β * skew + γ * migrationCost
```

---

# 21. Эпик 19. Adaptive Controller

```text
                   Metrics
                      ↓
              State Aggregator
                      ↓
                SystemState
                      ↓
             Workload Analyzer
                      ↓
            Bottleneck Detector
                      ↓
              Decision Policy
              /      |       \
             /       |        \
       ScaleOut   ScaleIn   Rebalance
             \       |        /
              Desired State
                      ↓
                Reconciler
                      ↓
                 Kubernetes
```

### Теория

- feedback control;
- reactive control;
- adaptive control;
- hysteresis;
- control-loop stability;
- delayed feedback;
- oscillation;
- convergence;
- state estimation;
- multi-metric autoscaling;
- bottleneck-aware scaling;
- resource provisioning;
- workload-aware scaling.

---

# 22. Эпик 20. Защита от oscillation

Без защиты возможно:

```text
4 → 5 → 4 → 5 → 4
```

### Теория

- hysteresis;
- cooldown;
- stabilization window;
- exponential moving average;
- control oscillation;
- delayed feedback.

Пример:

```text
ScaleOut: condition true for 30 sec
ScaleIn: condition true for 120 sec
Cooldown: 60 sec
```

---

# 23. Эпик 21. Reconciliation loop

```text
DesiredClusterState
       ↓
Reconciler
       ↓
ActualClusterState
```

### Теория

- declarative systems;
- desired state;
- actual state;
- reconciliation;
- eventual convergence;
- controller pattern;
- Kubernetes Operator pattern.

Концептуально:

```text
error(t) = desiredState(t) - actualState(t)
error(t) → 0
```

---

# 24. Эпик 22. Benchmark Framework

Пример конфигурации:

```yaml
workload: burst
duration: 10m
rps:
  initial: 500
  peak: 5000
distribution: zipf
scaling:
  policy: adaptive
```

### Теория

- experimental methodology;
- reproducibility;
- independent/dependent variables;
- control variables;
- warm-up;
- repetitions;
- confidence interval;
- mean/median;
- variance;
- statistical significance;
- outliers.

## Done

Эксперимент повторяем, параметры и raw data сохраняются, графики строятся автоматически.

---

# 25. Эпик 23. RQ1 — исследование partitioning

## Исследовательский вопрос

**Какой механизм логического распределения данных обеспечивает эффективное масштабирование и перераспределение нагрузки при изменении количества вычислительных узлов?**

Сравнить:

```text
hash(key) % nodeCount
consistent hashing
logical partitions + dynamic assignment
```

Сценарии:

```text
3 → 4
3 → 5
3 → 10
```

Метрики:

- moved keys / partitions;
- migration bytes;
- rebalance time;
- network traffic;
- p95/p99 during rebalance;
- unavailable time.

### Теория

- consistent hashing;
- rendezvous hashing;
- virtual nodes;
- partition placement;
- migration cost;
- data locality.

---

# 26. Эпик 24. RQ2 — исследование автомасштабирования

## Исследовательский вопрос

**Какой механизм масштабирования вычислительных узлов наиболее эффективно реагирует на изменение нагрузки по сравнению со статическим и основанным только на загрузке процессора масштабированием?**

Сравнить:

```text
Static
CPU-based / HPA
Adaptive multi-metric controller
```

Нагрузки:

- constant;
- ramp;
- burst;
- spike;
- periodic;
- gradual decrease.

Метрики:

- throughput;
- p50/p95/p99;
- node count;
- CPU;
- memory;
- Kafka lag;
- scaling event count;
- reaction time;
- convergence time;
- resource efficiency.

---

# 27. Эпик 25. RQ3 — Hot Partitions

## Исследовательский вопрос

**Насколько динамическое перераспределение логических разделов снижает деградацию производительности при неравномерной нагрузке?**

Нагрузки:

```text
Uniform
Zipf
Hot partition
Moving hot partition
```

Сравнить:

```text
without adaptive rebalancing
vs
with adaptive rebalancing
```

Метрики:

- throughput;
- p95/p99;
- CPU per node;
- skew;
- detection time;
- rebalance time;
- recovery time;
- migration cost.

### Теория

- Zipf distribution;
- Pareto principle;
- workload skew;
- hot keys;
- hot shards;
- dynamic load balancing.

---

# 28. Эпик 26. Fault experiments

Сценарии:

- kill node;
- kill node during migration;
- PostgreSQL slowdown;
- Kafka lag spike;
- Redis unavailable;
- Control Plane restart.

Метрики:

- availability;
- error rate;
- p99;
- recovery time;
- lost operations;
- duplicate operations;
- time to convergence.

### Теория

- fault tolerance;
- graceful degradation;
- recovery;
- idempotency;
- eventual consistency;
- CAP — в корректном контексте;
- partial failure.

---

# 29. Статья №1 — контрольная точка

## Рабочее название

**«Архитектура адаптивной распределённой системы хранения данных на основе динамически масштабируемых вычислительных узлов»**

## Когда уже можно начинать писать

### После завершения Эпика 13 — «Безопасная миграция раздела».

К этому моменту реализованы:

- вычислительный узел;
- PostgreSQL;
- логические разделы;
- несколько узлов;
- Router;
- Partition Assignment;
- Epoch;
- Kafka;
- базовое кэширование;
- наблюдаемость;
- Control Plane v1/v2;
- Kubernetes;
- baseline autoscaling;
- rebalancing;
- partition migration state machine;
- сценарии `3 → 5` и `5 → 2`.

Это минимальная точка, где архитектура уже подтверждена работающим прототипом, а не только схемой.

## Минимальные результаты статьи

- общая архитектурная диаграмма;
- структура вычислительного узла;
- структура Control Plane;
- модель Partition Assignment;
- lifecycle логического раздела;
- алгоритм scale-out;
- алгоритм scale-in;
- baseline latency/throughput;
- rebalance duration;
- число перенесённых разделов;
- p95/p99 во время rebalancing.

## Желательно завершить до отправки статьи

- **Эпик 14 — Kafka during rebalancing**;
- **Эпик 15 — Cache migration**;
- **Эпик 16 — базовая обработка отказов**.

Оптимальная точка подачи: **после эпиков 14–16**, но писать текст уже можно после эпика 13.

```text
EPIC 0
  ↓
...
  ↓
EPIC 13  ← ARTICLE 1: можно начинать писать
  ↓
EPIC 14–16 ← желательно закончить до подачи
```

---

# 30. Статья №2 — контрольная точка

## Рабочее название

**«Исследование адаптивного масштабирования распределённой системы хранения при неравномерной нагрузке»**

## Когда уже можно начинать писать

### После завершения Эпика 24 — RQ2 «Исследование автомасштабирования».

К этому моменту должны быть готовы:

- observability;
- Control Plane;
- Kubernetes;
- CPU/HPA baseline;
- многомерная модель состояния;
- bottleneck classification;
- rebalancing;
- hot-partition detector;
- load-aware placement;
- Adaptive Controller;
- hysteresis/cooldown;
- reconciliation loop;
- benchmark framework;
- воспроизводимые workload profiles;
- сравнение `Static vs CPU-based vs Adaptive`.

## Минимальная экспериментальная база

Три варианта системы:

```text
Baseline A: Static cluster
Baseline B: CPU-based autoscaling / HPA
Proposed: Adaptive multi-metric controller
```

Минимальные нагрузки:

```text
constant
ramp
burst
spike
```

Минимальные графики:

1. Incoming RPS vs time.
2. p95/p99 vs time.
3. Node count vs time.
4. CPU vs time.
5. Kafka lag vs time.
6. Reaction time.
7. Stabilization/convergence time.
8. Number of unnecessary scale events.
9. Resource utilization/efficiency.

## Желательно завершить до отправки статьи

- **Эпик 25 — RQ3 Hot Partitions**.

Именно он делает формулировку «при неравномерной нагрузке» особенно убедительной: можно показать ситуацию, когда общий CPU не описывает реальный bottleneck, а Adaptive Controller обнаруживает локальный skew и применяет Rebalance или `ScaleOut + Rebalance`.

Оптимальная точка подачи: **после эпика 25**.

```text
EPIC 17–21
   ↓
EPIC 22 — Benchmark Framework
   ↓
EPIC 23 — RQ1
   ↓
EPIC 24 ← ARTICLE 2: можно начинать писать
   ↓
EPIC 25 ← желательно закончить до подачи
   ↓
EPIC 26 ← дополнительный материал для ВКР
```

---

# 31. Оптимальное распределение содержания между статьями

## Статья №1 — архитектурная

Фокус:

- архитектурная модель;
- логические разделы;
- ownership;
- epoch;
- routing;
- Control Plane;
- масштабирование;
- rebalancing;
- safe migration;
- preliminary benchmarks.

Не нужно пытаться полностью доказать превосходство Adaptive Controller — это задача второй статьи.

## Статья №2 — экспериментальная

Фокус:

- experimental setup;
- workload profiles;
- Static baseline;
- CPU/HPA baseline;
- Adaptive Controller;
- burst/spike/ramp;
- Zipf/skew/hot partition;
- reaction time;
- p99;
- resource efficiency;
- rebalancing effectiveness.

---

# 32. Что оставить для полной ВКР

Не обязательно помещать всё в статьи. В ВКР подробно раскрываются:

- анализ аналогов;
- формальный аппарат;
- проектирование API;
- внутреннее устройство вычислительного узла;
- обоснование Kafka/Redis/PostgreSQL;
- deployment;
- reconciliation;
- отказоустойчивость;
- fault experiments;
- полные RQ1/RQ2/RQ3;
- статистическая обработка;
- промежуточные версии алгоритмов;
- отрицательные результаты;
- ограничения метода;
- направления дальнейшей работы.

---

# 33. Карта проекта одним экраном

```text
Этап A. Фундамент
────────────────────────────────────
EPIC 0   Формальная модель
EPIC 1   Один Node + PostgreSQL
EPIC 2   Logical partitions
EPIC 3   Несколько Nodes
EPIC 4   Epoch

Этап B. Data Plane
────────────────────────────────────
EPIC 5   Kafka
EPIC 6   Cache
EPIC 7   Observability

Этап C. Control Plane
────────────────────────────────────
EPIC 8   Control Plane v1
EPIC 9   Kubernetes
EPIC 10  Basic autoscaling
EPIC 11  Bottleneck classification

Этап D. Dynamic topology
────────────────────────────────────
EPIC 12  Rebalancing
EPIC 13  Safe partition migration
───────────── ARTICLE 1 CAN START ─────────────
EPIC 14  Kafka + rebalancing
EPIC 15  Cache migration
EPIC 16  Failures

Этап E. Adaptive control
────────────────────────────────────
EPIC 17  Hot Partition Detection
EPIC 18  Load-aware Rebalancing
EPIC 19  Adaptive Controller
EPIC 20  Oscillation Protection
EPIC 21  Reconciliation Loop

Этап F. Research
────────────────────────────────────
EPIC 22  Benchmark Framework
EPIC 23  RQ1 Partitioning
EPIC 24  RQ2 Autoscaling
───────────── ARTICLE 2 CAN START ─────────────
EPIC 25  RQ3 Hot Partitions
EPIC 26  Fault Experiments
```

---

# 34. Первый практический спринт

До Kafka, Redis, Kubernetes и Control Plane формализовать:

```text
1. Data model
2. Compute node model
3. Logical partition model
4. Partition Assignment
5. Epoch model
6. Routing algorithm
7. System invariants
8. Partition lifecycle
9. 3 nodes → 5 nodes
10. 5 nodes → 2 nodes
11. Node crash
12. Node crash during migration
```

Главная цепочка управления:

```text
SystemState
    ↓
ScalingDecision
    ↓
DesiredClusterState
    ↓
PartitionAssignment
    ↓
Reconciliation
```

Первый полноценный технический документ проекта должен формализовать lifecycle логического раздела при:

```text
3 nodes → 5 nodes → 2 nodes
```

с учётом:

- ownership;
- epoch;
- migration;
- cache invalidation;
- cache warm-up;
- Kafka offsets;
- duplicate operations;
- lost operations;
- graceful draining;
- failure during rebalancing.

Это основной entry point перед масштабной реализацией.
# 29. Обновлённая стратегия публикаций

Статьи должны появляться как самостоятельные ранние исследовательские результаты, а не ждать почти полной реализации ВКР.

## Статья №1 — примерно 20–25% проекта

**Тема:** «Исследование методов логического распределения данных в динамически масштабируемой распределённой системе».

Минимально необходимы:

- формальная модель;
- Node + PostgreSQL;
- Logical Partitions;
- Multi-node + Router;
- Partition Assignment;
- Epoch;
- отдельный partitioning benchmark/simulator.

Исследовательский вопрос: какой механизм логического распределения данных эффективнее при изменении количества вычислительных узлов?

Сравнить:

```text
A. hash(key) % nodeCount
B. consistent hashing
C. fixed logical partitions + dynamic PartitionAssignment
```

Сценарии:

```text
1 000 000 keys
128 logical partitions
3 → 4 nodes
3 → 5 nodes
5 → 3 nodes
5 → 10 nodes
```

Метрики: доля перемещаемых ключей, число переназначаемых разделов, потенциальный объём миграции, max/mean load, coefficient of variation, время расчёта новой карты.

Теория: sharding, hash partitioning, consistent hashing, rendezvous hashing, virtual nodes, logical partitions, assignment, data locality, load distribution, migration cost.

```text
A0 → A1 → A2 → A3 → A4
                     ↓
                ARTICLE 1
```

---

## Статья №2 — примерно 40–45% проекта

**Тема:** «Исследование многокритериального масштабирования вычислительных узлов распределённой системы».

Альтернатива: «Исследование методов масштабирования вычислительных узлов распределённой системы при динамической нагрузке».

Минимально необходимы:

- Observability;
- Kubernetes;
- Control Plane v1;
- Static baseline;
- CPU/HPA baseline;
- ранняя multi-metric scaling policy;
- benchmark workloads.

На этом этапе НЕ обязательны safe migration, hot-partition rebalancing, fault recovery, predictive scaling и ML.

Исследовательский вопрос: позволяет ли использование нескольких характеристик состояния системы эффективнее реагировать на динамическую нагрузку, чем статическое размещение ресурсов и CPU-only scaling?

Сравнить:

```text
Static
CPU-based / HPA
Multi-metric
```

Ранний контроллер использует:

```text
CPU
RPS
p99
active requests / queue size
DB latency
(+ Kafka lag, если Kafka уже введена)
        ↓
ScaleOut / ScaleIn / Maintain
```

Нагрузки:

```text
constant
ramp
burst
spike
periodic
```

Метрики: throughput, p95/p99, CPU, node count, reaction time, stabilization time, scaling events, resource consumption, error rate.

Дополнительная метрика:

```text
ResourceEfficiency = processedRequests / nodeSeconds
```

Теория: horizontal autoscaling, Kubernetes HPA, threshold control, feedback loop, hysteresis, cooldown, stabilization window, saturation, bottleneck, queueing basics, delayed feedback, oscillation, multi-metric autoscaling.

```text
B1 → B2 → B3 → B4 → B5 → B6
                          ↓
                     ARTICLE 2
```

---

# 30. Новая каноническая последовательность реализации

## Этап A. Фундамент — 0–25%

### A0. Формальная модель

Теория: distributed systems, compute/storage separation, ownership, partitioning, routing, scaling vs rebalancing, invariants.

Результат:

```text
DataModel
Node
PartitionId
NodeId
PartitionAssignment
Epoch
```

### A1. Один Node + PostgreSQL

Теория: Go concurrency, REST/gRPC, PostgreSQL transactions, MVCC, connection pooling, latency/throughput, benchmarking basics.

### A2. Logical Partitions

Теория: hash partitioning, consistent hashing, rendezvous hashing, virtual partitions, sharding.

### A3. Multi-node + Router

Теория: ownership, routing, topology, service discovery, metadata.

```text
key → PartitionId → PartitionAssignment → Node
```

### A4. Assignment + Epoch

Теория: topology version, generation, logical clocks, fencing token, stale state.

### A5. Partitioning Research Harness

Реализовать симулятор/benchmark для сравнения способов распределения и изменения топологии.

**После A5 — подготовка и подача статьи №1.**

---

# 31. Этап B. Управление и раннее масштабирование — 25–45%

## B1. Observability

Теория: metrics/logs/traces, RED, USE, histograms, counters, gauges, percentiles, time series, moving averages, Prometheus.

Метрики:

```text
RPS
p50/p95/p99
CPU/RAM
active requests
queue size
DB latency
partition load
```

## B2. Kubernetes

Теория: Pod, Deployment, ReplicaSet, Service, readiness/liveness, requests/limits, scheduler.

## B3. Control Plane v1

```text
Metrics → State Aggregator → SystemState → Decision
```

Теория: control plane/data plane, feedback loop, observable state, sampling interval, aggregation.

## B4. Static + CPU-based Scaling

Теория: HPA, target utilization, threshold control, hysteresis, cooldown, stabilization, oscillation.

## B5. Early Multi-Metric Scaling

Вход:

```text
CPU
RPS
p99
queue size
DB latency
```

Выход:

```text
ScaleOut
ScaleIn
Maintain
```

Теория: multi-metric autoscaling, saturation, bottleneck analysis, feedback control, delayed reaction, resource efficiency.

## B6. Early Benchmark Workloads

Сравнить Static vs CPU-based vs Multi-metric на constant/ramp/burst/spike/periodic нагрузках.

**После B6 — подготовка и подача статьи №2.**

---

# 32. Этап C. Расширение Data Plane — 45–55%

## C1. Kafka

Теория: broker/topic/partition, producer/consumer, consumer groups, offsets, ordering, batching, delivery semantics.

## C2. Idempotency

Теория: at-least-once, duplicate delivery, idempotent consumer, deduplication, inbox/outbox, retry, DLQ.

## C3. L1 Cache

Теория: cache-aside, TTL, LRU/LFU, invalidation, stale data, coherence.

## C4. Redis L2

Теория: shared cache, local vs distributed cache, invalidation, network overhead, hit ratio.

Kafka и Redis намеренно не блокируют ранние публикации.

---

# 33. Этап D. Динамическая топология — 55–70%

## D1. Rebalancing

Теория: repartitioning, minimal movement, placement, migration cost, consistent/rendezvous hashing, bin packing basics.

## D2. Partition State Machine

```text
OWNED → PREPARING → TRANSFERRING → ACTIVATING → OWNED
```

Теория: FSM, ownership transfer, lease, fencing, epoch, split brain.

## D3. Graceful Draining

Сценарии `3 → 5` и `5 → 2`.

Теория: in-flight operations, graceful shutdown, draining, quiescence, acknowledgement.

## D4. Kafka During Migration

Теория: offset ownership, consumer rebalance, duplicates, ordering, fencing, transactional consumption.

## D5. Cache Migration

Теория: cold cache, warm-up, invalidation, prewarming.

## D6. Failure Recovery

Теория: crash-stop/recovery, failure detector, heartbeat, timeout, partial failure, recovery state machine, conceptual consensus/quorum/2PC.

---

# 34. Этап E. Полноценное адаптивное управление — 70–85%

## E1. Hot Partition Detection

Теория: workload skew, Zipf, coefficient of variation, max/mean, standard deviation, hot keys/shards.

```text
Skew = max(partitionLoad) / mean(partitionLoad)
```

## E2. Load-Aware Placement

Теория: dynamic load balancing, scheduling, bin packing, greedy placement, migration cost, cost functions.

```text
Cost = α·latency + β·skew + γ·migrationCost
```

## E3. Bottleneck Classification

```text
Healthy
ComputeBound
QueueBound
DatabaseBound
HotPartition
Overprovisioned
```

Теория: rule-based classification, decision tables, workload classification, saturation, backpressure.

## E4. Adaptive Controller

```text
Metrics
 ↓
SystemState
 ↓
BottleneckDetector
 ↓
DecisionPolicy
 ↓
Scale / Rebalance / Maintain
```

Теория: adaptive/reactive control, multi-variable feedback, stability, convergence, workload-aware scaling.

## E5. Oscillation Protection

Теория: hysteresis, cooldown, stabilization windows, EWMA, delayed feedback, control oscillation.

## E6. Reconciliation Loop

```text
DesiredClusterState → Reconciler → ActualClusterState
```

Теория: desired/actual state, declarative control, reconciliation, eventual convergence, controller pattern.

---

# 35. Этап F. Финальная экспериментальная часть ВКР — 85–100%

## F1. Full Benchmark Framework

Теория: reproducibility, variables, warm-up, repetitions, confidence intervals, variance, statistical significance, outliers.

Каждый запуск сохраняет configuration, environment, raw metrics, results и plots.

## F2. RQ1 Extended

Расширить статью №1 экспериментами на реальной системе: migration, rebalance time, network traffic, p99 during rebalance, влияние количества logical partitions.

## F3. RQ2 Extended

Сравнить:

```text
Static
CPU/HPA
Early Multi-Metric
Full Adaptive
```

## F4. RQ3 Hot Partitions

Нагрузки:

```text
Uniform
Zipf
Hot Partition
Moving Hot Partition
```

Сравнить систему без adaptive rebalancing и с ним.

## F5. Fault Experiments

Сценарии: kill node, failure during migration, PostgreSQL slowdown, Kafka lag spike, Redis unavailable, Control Plane restart.

Метрики: availability, errors, p99, recovery time, duplicates, lost operations, convergence time.

---

# 36. Связь статей и ВКР

```text
                 ВКР
                  │
        ┌─────────┴─────────┐
        │                   │
    ARTICLE 1           ARTICLE 2
        │                   │
   Partitioning          Autoscaling
        │                   │
        └─────────┬─────────┘
                  │
         Full Adaptive System
                  │
       Dynamic Rebalancing
       Hot Partition Detection
       Bottleneck Classification
       Failure Recovery
```

Статья №1 отвечает:

> Как логически распределять данные при изменяемой топологии вычислительных узлов?

Статья №2 отвечает:

> Как принимать решение об изменении числа вычислительных узлов при динамической нагрузке?

Полная ВКР отвечает:

> Как совместно управлять количеством вычислительных узлов и размещением логических разделов на основе текущего многомерного состояния распределённой системы?

---

# 37. Временная карта

```text
0%                                                    100%
│────────────────────────────────────────────────────────│

████████████
A. Fundamentals
          │
          └── ARTICLE 1 (~20–25%)

            █████████████
            B. Control Plane + Early Scaling
                         │
                         └── ARTICLE 2 (~40–45%)

                          ███████
                          C. Kafka + Cache

                                ██████████
                                D. Dynamic Topology

                                          ██████████
                                          E. Adaptive Control

                                                    ███████
                                                    F. Research
                                                       ↓
                                                      ВКР
```

---

# 38. Текущий entry point

Первый технический пакет:

```text
1. Data model
2. Compute Node model
3. Logical Partition model
4. Partition Assignment
5. Epoch
6. Routing algorithm
7. System invariants
8. Topology model
```

После него:

```text
Node + PostgreSQL
        ↓
Logical Partitions
        ↓
Multi-node Router
        ↓
Assignment + Epoch
        ↓
Partitioning Benchmark
        ↓
ARTICLE 1
```

Ближайшая крупная цель — не Kafka и не Kubernetes, а довести фундамент до состояния, достаточного для первого самостоятельного исследования и первой статьи.
