# Исследовательские эпики: каталог задач

Это детализация исходного исследовательского плана, а не отчёт о реализации или очередность работ. Номера эпиков и Feature сохранены как идентификаторы теоретических задач. Канонический порядок задаёт [roadmap](../roadmap.md); текущий рабочий эпик имеет отдельную [спецификацию](epic-1/README.md), которая приоритетнее исходного описания базового узла ниже.

При выделении следующего рабочего эпика сюда не добавляется второй статус реализации: его границы, API и проверки оформляются в собственной директории. Уже реализованные механизмы описывает [текущая архитектура](../architecture.md).

## Entry point — формализация системы

### Цель

До основной реализации определить сущности системы, границы ответственности компонентов, инварианты и сценарии изменения топологии.

### Feature 0.1. Формальная модель системы

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

### Feature 0.2. Модель данных

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

### Feature 0.3. Инварианты

1. Каждый активный логический раздел имеет владельца.
2. Операции записи принимает только актуальный владелец.
3. Версия назначения владельца монотонно возрастает.
4. Повторное выполнение операции не нарушает корректность состояния.
5. После завершения перераспределения каждый раздел имеет ровно одного активного владельца.
6. Постоянное состояние не зависит от жизненного цикла вычислительного узла.

### Результат

Отдельный технический документ с моделями данных, узла, логического раздела, ownership, routing, epoch и базовыми сценариями `3 → 5 → 2 nodes`.

---

## Эпик 1. Базовый вычислительный узел

### Цель

```text
Client
   ↓
Node
   ↓
PostgreSQL
```

Без Kafka, Redis, Kubernetes и автомасштабирования.

### Feature 1.1. API вычислительного узла

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

### Feature 1.2. PostgreSQL Repository

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

### Feature 1.3. Первый нагрузочный тест

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

### Done

Есть воспроизводимый baseline и графики latency/throughput от offered load.

---

## Эпик 2. Логическое разделение данных

### Feature 2.1. Hash partitioning

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

### Feature 2.2. Доменная модель разделов

Сущности:

```text
PartitionId
PartitionCount
Partitioner
```

### Done

Количество логических разделов не зависит от количества вычислительных узлов.

---

## Эпик 3. Несколько вычислительных узлов

### Feature 3.1. Partition Assignment

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

### Feature 3.2. Router

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

### Done

Запрос всегда направляется владельцу соответствующего логического раздела.

---

## Эпик 4. Версионирование топологии

### Feature 4.1. ClusterEpoch / PartitionEpoch

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

### Done

Устаревшая карта размещения не позволяет выполнить запись от имени старого владельца.

---

## Эпик 5. Kafka и асинхронная обработка

### Feature 5.1. Producer / Consumer

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

### Feature 5.2. Idempotency

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

### Done

Повторная доставка не нарушает состояние данных.

---

## Эпик 6. Кэширование

### Feature 6.1. L1 cache

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

### Feature 6.2. Redis L2

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

### Feature 6.3. Эксперимент

Сравнить:

```text
PostgreSQL only
L1 + PostgreSQL
L1 + Redis + PostgreSQL
```

Метрики: p95/p99, DB QPS, cache hit ratio, network, memory.

---

## Эпик 7. Наблюдаемость

### Feature 7.1. Метрики узлов

- requests/sec;
- p50/p95/p99;
- CPU;
- memory;
- active requests;
- queue size.

### Feature 7.2. Метрики Kafka

- consumer lag;
- messages/sec;
- produce latency;
- consume latency.

### Feature 7.3. Метрики PostgreSQL

- query latency;
- connections;
- QPS.

### Feature 7.4. Метрики логических разделов

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

### Done

По метрикам можно определить текущее состояние кластера и вероятный bottleneck.

---

## Эпик 8. Control Plane v1

### Feature 8.1. SystemState

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

### Feature 8.2. Базовая функция решения

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

## Эпик 9. Kubernetes

### Feature 9.1. Deployment

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

### Feature 9.2. HPA baseline

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

## Эпик 10. Control Plane v2 — базовое автомасштабирование

### Feature 10.1. Threshold scaling

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

### Feature 10.2. Многомерное состояние

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

## Эпик 11. Классификация состояния системы

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

## Эпик 12. Rebalancing

### Feature 12.1. Новый Partition Assignment

### Теория

- repartitioning;
- rebalancing;
- consistent hashing;
- rendezvous hashing;
- virtual shards;
- minimal movement;
- load-aware partition assignment;
- bin packing — обзорно.

### Feature 12.2. Scale-out `3 → 5`

1. Control Plane принимает решение.
2. Desired replicas увеличивается.
3. Kubernetes запускает Pods.
4. Ожидается readiness.
5. Новый узел регистрируется.
6. Рассчитывается новая карта разделов.
7. Выполняется rebalancing.
8. Старые владельцы освобождают разделы.
9. Новая карта активируется.

### Feature 12.3. Scale-in `5 → 2`

1. Выбрать удаляемые узлы.
2. Назначить их разделы другим узлам.
3. Перевести узлы в DRAINING.
4. Не направлять новые операции.
5. Завершить in-flight операции.
6. Подтвердить transfer ownership.
7. Уменьшить replicas.

---

## Эпик 13. Безопасная миграция раздела

### Feature 13.1. Partition State Machine

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

### Feature 13.2. Graceful draining

### Теория

- graceful shutdown;
- draining;
- in-flight requests;
- quiescence;
- handoff;
- acknowledgement;
- timeout;
- retry.

### Done

`3 → 5` и `5 → 2` выполняются без неопределённого ownership.

---

## Эпик 14. Kafka во время rebalancing

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

## Эпик 15. Cache migration

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

## Эпик 16. Отказы

### Feature 16.1. Node crash

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

### Feature 16.2. Failure during migration

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

## Эпик 17. Hot Partition Detection

### Feature 17.1. Skew metric

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

### Feature 17.2. Detector

Определить threshold, временное окно, минимальную длительность перегрева и защиту от false positive.

---

## Эпик 18. Load-aware rebalancing

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

## Эпик 19. Adaptive Controller

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

## Эпик 20. Защита от oscillation

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

## Эпик 21. Reconciliation loop

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

## Эпик 22. Benchmark Framework

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

### Done

Эксперимент повторяем, параметры и raw data сохраняются, графики строятся автоматически.

---

## Эпик 23. RQ1 — исследование partitioning

### Исследовательский вопрос

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

## Эпик 24. RQ2 — исследование автомасштабирования

### Исследовательский вопрос

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

## Эпик 25. RQ3 — Hot Partitions

### Исследовательский вопрос

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

## Эпик 26. Fault experiments

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
