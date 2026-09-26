# Roadmap

Порядок развития: **A → B → C → D → E → F**. Проценты обозначают ориентиры исследовательского плана, а не измеренный прогресс разработки. Детализация будущих работ — в [каталоге эпиков](epics/planned.md), теоретическая модель — в [описании ВКР](vkrm.md).

## Текущее положение

[Эпик 1](epics/epic-1/README.md) объединяет документный API, PostgreSQL, статический Router, наблюдаемость и локальный нагрузочный стенд. Это практический фундамент этапа A с ранней наблюдаемостью из этапа B. Текущая контейнерная проверка и короткие диагностические прогоны выполнены 26.09.2026; результаты и оставшиеся исследовательские ограничения перечислены в [валидации](epics/epic-1/validation.md).

Следующий исследовательский шаг после проверки стенда — воспроизводимое сравнение стратегий partitioning и формализация гарантий assignment/epoch. Локальная проверка epoch ещё не является fencing. Control plane, динамическая топология, Scala DSL и автоскейлинг не реализованы. Способ развёртывания Scala DSL остаётся открытым.

Номера исходных исследовательских эпиков не задают очередность разработки: наблюдаемость перенесена в текущий эпик, Kafka и кэши — после ранних исследований. Новые рабочие эпики получают собственные границы и критерии проверки в `docs/epics/`.

## Стратегия публикаций

Статьи должны появляться как самостоятельные ранние исследовательские результаты, а не ждать почти полной реализации ВКР.

### Статья №1 — примерно 20–25% проекта

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

### Статья №2 — примерно 40–45% проекта

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

## Этап A. Фундамент и partitioning — 0–25%

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

## Этап B. Управление и раннее масштабирование — 25–45%

### B1. Observability

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

### B2. Kubernetes

Теория: Pod, Deployment, ReplicaSet, Service, readiness/liveness, requests/limits, scheduler.

### B3. Control Plane v1

```text
Metrics → State Aggregator → SystemState → Decision
```

Теория: control plane/data plane, feedback loop, observable state, sampling interval, aggregation.

### B4. Static + CPU-based Scaling

Теория: HPA, target utilization, threshold control, hysteresis, cooldown, stabilization, oscillation.

### B5. Early Multi-Metric Scaling

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

### B6. Early Benchmark Workloads

Сравнить Static vs CPU-based vs Multi-metric на constant/ramp/burst/spike/periodic нагрузках.

**После B6 — подготовка и подача статьи №2.**

---

## Этап C. Расширение Data Plane — 45–55%

### C1. Kafka

Теория: broker/topic/partition, producer/consumer, consumer groups, offsets, ordering, batching, delivery semantics.

### C2. Idempotency

Теория: at-least-once, duplicate delivery, idempotent consumer, deduplication, inbox/outbox, retry, DLQ.

### C3. L1 Cache

Теория: cache-aside, TTL, LRU/LFU, invalidation, stale data, coherence.

### C4. Redis L2

Теория: shared cache, local vs distributed cache, invalidation, network overhead, hit ratio.

Kafka и Redis намеренно не блокируют ранние публикации.

---

## Этап D. Динамическая топология — 55–70%

### D1. Rebalancing

Теория: repartitioning, minimal movement, placement, migration cost, consistent/rendezvous hashing, bin packing basics.

### D2. Partition State Machine

```text
OWNED → PREPARING → TRANSFERRING → ACTIVATING → OWNED
```

Теория: FSM, ownership transfer, lease, fencing, epoch, split brain.

### D3. Graceful Draining

Сценарии `3 → 5` и `5 → 2`.

Теория: in-flight operations, graceful shutdown, draining, quiescence, acknowledgement.

### D4. Kafka During Migration

Теория: offset ownership, consumer rebalance, duplicates, ordering, fencing, transactional consumption.

### D5. Cache Migration

Теория: cold cache, warm-up, invalidation, prewarming.

### D6. Failure Recovery

Теория: crash-stop/recovery, failure detector, heartbeat, timeout, partial failure, recovery state machine, conceptual consensus/quorum/2PC.

---

## Этап E. Полноценное адаптивное управление — 70–85%

### E1. Hot Partition Detection

Теория: workload skew, Zipf, coefficient of variation, max/mean, standard deviation, hot keys/shards.

```text
Skew = max(partitionLoad) / mean(partitionLoad)
```

### E2. Load-Aware Placement

Теория: dynamic load balancing, scheduling, bin packing, greedy placement, migration cost, cost functions.

```text
Cost = α·latency + β·skew + γ·migrationCost
```

### E3. Bottleneck Classification

```text
Healthy
ComputeBound
QueueBound
DatabaseBound
HotPartition
Overprovisioned
```

Теория: rule-based classification, decision tables, workload classification, saturation, backpressure.

### E4. Adaptive Controller

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

### E5. Oscillation Protection

Теория: hysteresis, cooldown, stabilization windows, EWMA, delayed feedback, control oscillation.

### E6. Reconciliation Loop

```text
DesiredClusterState → Reconciler → ActualClusterState
```

Теория: desired/actual state, declarative control, reconciliation, eventual convergence, controller pattern.

---

## Этап F. Финальная экспериментальная часть ВКР — 85–100%

### F1. Full Benchmark Framework

Теория: reproducibility, variables, warm-up, repetitions, confidence intervals, variance, statistical significance, outliers.

Каждый запуск сохраняет configuration, environment, raw metrics, results и plots.

### F2. RQ1 Extended

Расширить статью №1 экспериментами на реальной системе: migration, rebalance time, network traffic, p99 during rebalance, влияние количества logical partitions.

### F3. RQ2 Extended

Сравнить:

```text
Static
CPU/HPA
Early Multi-Metric
Full Adaptive
```

### F4. RQ3 Hot Partitions

Нагрузки:

```text
Uniform
Zipf
Hot Partition
Moving Hot Partition
```

Сравнить систему без adaptive rebalancing и с ним.

### F5. Fault Experiments

Сценарии: kill node, failure during migration, PostgreSQL slowdown, Kafka lag spike, Redis unavailable, Control Plane restart.

Метрики: availability, errors, p99, recovery time, duplicates, lost operations, convergence time.

---

## Связь статей и ВКР

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

## Временная карта

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
