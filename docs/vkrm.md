# ВКРМ — адаптивная распределённая система хранения и обработки данных

> Теоретическое описание ВКР: целевая модель, исследовательские вопросы и обоснования. Упоминание компонента здесь не означает его реализацию. Актуальный порядок работ задаёт [roadmap](roadmap.md), реализованное поведение — [архитектура](architecture.md), границы текущей поставки — [эпик 1](epics/epic-1/README.md). Встроенные последовательности этапов читаются как теоретический контекст, а не отдельный план разработки.

## 0. Статус документа

Это обновлённый единый технический контекст проекта ВКРМ. Он предназначен для последовательной реализации, передачи контекста AI/Codex-агенту, проведения экспериментов и подготовки публикаций.

**Целевая дата завершения ВКРМ:** 01.03.2027  
**Планируемый период основных работ:** сентябрь 2026 — февраль 2027  
**Планируемые статьи:** 2 статьи по 7–10 страниц.  
**Целевые публикационные точки:** статья №1 примерно на 20–25% реализации, статья №2 примерно на 40–45%.

**Планируемый объём ВКР:** около 100 страниц:
- ~50 страниц — исследовательская часть;
- ~40 страниц — конструкторская часть;
- ~10 страниц — технологическая/вспомогательная часть.

### Ключевое изменение плана

Kafka и Redis больше не являются блокирующими технологиями для ранних публикаций. Реализация организуется так:

```text
A. Fundamentals
   → ARTICLE 1 (~20–25%)

B. Observability + Kubernetes + Control Plane + Early Scaling
   → ARTICLE 2 (~40–45%)

C. Kafka + Cache
D. Safe Dynamic Topology
E. Full Adaptive Control
F. Extended Research → ВКР
```

---

# 1. Рабочая идея ВКРМ

Разработать распределённую систему хранения и обработки данных, состоящую из динамически масштабируемых **вычислительных узлов**, с отдельным **Control Plane**, который анализирует состояние системы и адаптивно управляет количеством вычислительных единиц и распределением логических разделов.

Основная идея:

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
        | N1|       | N2|       | N3|   ... N-N
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

**Ключевой научный фокус:** не создание «нового Ignite», а исследование и реализация механизма адаптивного управления распределёнными вычислительные узлы при динамической и неравномерной нагрузке.

Space-Based Architecture используется как архитектурное вдохновение и предмет анализа, но не должна сама по себе быть заявлена как научная новизна.

---

# 2. Тема ВКРМ

Основной вариант официальной темы:

> **Распределённая система хранения и обработки данных с адаптивным масштабированием вычислительных узлов**

Рабочая исследовательская формулировка предмета:

> **Метод адаптивного управления вычислительными ресурсами распределённой системы хранения данных при динамической и неравномерной нагрузке**

В официальном названии используются русские термины. Названия конкретных технологий (PostgreSQL, Kafka, Redis, Kubernetes, Prometheus и т. п.) сохраняются в технической документации.

**Определение:** вычислительный узел — программный компонент системы, выполняющий обработку запросов и операций над закреплённым за ним набором логических разделов данных.

---

# 3. Научная идея

## 3.1. Проблема

В распределённых системах хранения и обработки данных количество вычислительных ресурсов должно соответствовать текущей нагрузке.

Простое Cвычислительный узел-based autoscaling имеет ограничения:

- Cвычислительный узел не всегда является причиной деградации;
- система может быть ограничена Kafka lag;
- система может быть ограничена PostgreSQL;
- может возникать hot partition;
- рост числа вычислительный узел не всегда улучшает производительность;
- масштабирование может запаздывать относительно burst-нагрузки;
- равномерное распределение ресурсов не гарантирует равномерную нагрузку.

## 3.2. Предлагаемый подход

Control Plane получает многомерное состояние системы:

- Cвычислительный узел;
- memory;
- RPS;
- p50/p95/p99 latency;
- Kafka lag;
- Kafka throughput;
- PostgreSQL latency;
- PostgreSQL connections/QPS;
- Redis hit ratio;
- partition load/skew;
- количество активных вычислительный узел.

На основании этого строится состояние системы и принимается решение:

```text
Telemetry
    |
    v
State Aggregation
    |
    v
Workload / Bottleneck Analysis
    |
    v
SystemState
    |
    v
Scaling / Rebalancing Policy
    |
    v
ScalingDecision
    |
    v
Desired State
    |
    v
Reconciler
    |
    v
Kubernetes
```

Пример:

```text
Cвычислительный узел = 45%
Kafka lag = HIGH
p99 = HIGH
DB latency = NORMAL

=> QueueBound
=> ScaleOut
```

Другой пример:

```text
Cвычислительный узел = 40%
Kafka lag = HIGH
p99 = HIGH
PostgreSQL latency = VERY HIGH

=> DatabaseBound
=> увеличение вычислительный узел может не помочь
=> throttle / batching / сохранение числа вычислительный узел
```

Третий:

```text
N2:
P1 = 100 req/s
P2 = 110 req/s
P3 = 1000 req/s
P4 = 90 req/s

=> HotPartition
=> rebalance / перенос partition
```

---

# 4. Исследовательские вопросы

Планируется три основных исследовательских вопроса.

## RQ1. Распределение данных

**Какой механизм логического распределения данных обеспечивает эффективное масштабирование и перераспределение нагрузки при изменении количества вычислительные узлы?**

Исследовать:

- hash partitioning;
- range partitioning;
- логические разделы + dynamic assignment.

Предпочтительный вариант для системы:

```text
128 логические разделы
        |
        +-- N1
        +-- N2
        +-- N3
        +-- N4
```

При scale-out:

```text
128 логические разделы
        |
        +-- N1
        +-- N2
        +-- N3
        +-- N4
        +-- N5
        +-- N6
```

Количество логических разделов остаётся постоянным, меняется ownership/assignment.

Измерять:

- объём миграции;
- количество перемещённых партиций;
- время rebalance;
- latency во время rebalance;
- downtime;
- network overhead;
- влияние количества логические разделы.

---

## RQ2. Адаптивное масштабирование

**Какой механизм масштабирования вычислительные узлы наиболее эффективно реагирует на изменение и неравномерность нагрузки по сравнению со статическим и Cвычислительный узел-based масштабированием?**

Сравнить минимум:

### Baseline A — Static

Фиксированное количество вычислительный узел.

### Baseline B — Cвычислительный узел-based autoscaling

Условно:

```text
Cвычислительный узел > threshold
    |
    v
scale out
```

Это аналогия с обычным HPA-подходом.

### Proposed — Multi-metric / bottleneck-aware controller

Использует:

- Cвычислительный узел;
- Kafka lag;
- p99;
- DB pressure;
- partition skew;
- cache metrics.

Исследовать:

- реакцию на burst;
- latency;
- throughput;
- resource efficiency;
- время принятия решения;
- время convergence;
- количество лишних scale-out/scale-in операций.

---

## RQ3. Неравномерная нагрузка / hot partitions

**Насколько адаптивное перераспределение логические разделы снижает деградацию производительности при неравномерной нагрузке?**

Workloads:

1. Uniform:
```text
P1 █████
P2 █████
P3 █████
P4 █████
```

2. Zipf/skew:
```text
P1 ███████████████
P2 ███████
P3 ███
P4 ██
```

3. Hot partition:
```text
P1 █
P2 █
P3 ███████████████████
P4 █
```

4. Moving hot partition.

Измерять:

- RPS;
- p50/p95/p99;
- Cвычислительный узел;
- Kafka lag;
- partition skew;
- время обнаружения hot partition;
- время rebalance;
- recovery time.

---

# 5. Гипотеза исследования

Рабочая формулировка:

> Адаптивное управление вычислительные узлы на основе многомерного состояния распределённой системы и динамического перераспределения логических разделов позволяет снизить tail latency и/или повысить эффективность использования ресурсов по сравнению со статическим и Cвычислительный узел-only масштабированием при динамической и неравномерной нагрузке.

Гипотеза не требует превосходства предлагаемого подхода во всех сценариях. Важно показать, где и почему он даёт преимущество, а где дополнительные механизмы не дают выигрыша.

---

# 6. Границы научной новизны

Не заявлять:

- «создан новый Apache Ignite»;
- «создана лучшая распределённая БД»;
- «система надёжнее всех существующих решений»;
- «полная замена Ignite/Redis/PostgreSQL».

Основная потенциальная новизна:

1. Модель многомерного состояния вычислительного узла/кластера.
2. Bottleneck-aware классификация состояния системы.
3. Адаптивное решение о scale-out/scale-in/maintain.
4. Связка adaptive scaling + dynamic логический раздел assignment.
5. Экспериментальная оценка поведения при skew/hot partitions/bursts.
6. Возможно, алгоритм безопасного rebalancing с ownership/epoch и контролем состояния.

---

# 7. Архитектура

## 7.1. Основные слои

### Storage layer

- PostgreSQL — authoritative persistent storage.
- Redis — опциональный shared L2 cache.

### Processing layer

- вычислительные узлы.
- local L1 cache.
- Kafka consumer/producer.
- routing.
- batching.
- PostgreSQL client.
- metrics.

### Control layer

- metrics collection;
- state aggregation;
- bottleneck detection;
- scaling policy;
- rebalancing policy;
- desired state;
- reconciliation;
- Kubernetes adapter.

### Experimental layer

- workload generator;
- benchmark runner;
- metrics collection;
- result analysis;
- plots.

---

# 8. вычислительный узел

вычислительный узел — основная вычислительная единица data plane.

Возможная структура:

```text
+----------------------------------+
|         вычислительный узел         |
|                                  |
|  API / Request Handler           |
|          |                       |
|       Router                     |
|          |                       |
|   Partition Manager              |
|          |                       |
|   +------+---------+              |
|   |                |              |
| L1 Cache        Kafka             |
|   |                |              |
|   +--------+-------+              |
|            |                      |
|       PostgreSQL                  |
|                                  |
|       Metrics                    |
+----------------------------------+
```

Предпочтительно сделать вычислительный узел максимально stateless с точки зрения оркестрации:

- durable state — PostgreSQL;
- transport/buffering — Kafka;
- shared cache — Redis;
- local cache — временное состояние;
- ownership metadata — отдельный metadata layer.

---

# 9. Cache architecture

Не делать полную mesh-репликацию кэша между всеми вычислительный узел.

Предпочтительная схема:

```text
GET
 |
 v
L1: вычислительный узел local memory
 |
 | miss
 v
L2: Redis
 |
 | miss
 v
L3: PostgreSQL
 |
 v
populate Redis
 |
 v
populate L1
```

Причины отказа от full replication:

- memory explosion;
- network overhead;
- сложная invalidation;
- дорогой scale-out;
- большое количество coherence traffic.

MVP:

- L1 local cache;
- PostgreSQL.

Следующий этап:

- Redis L2.

Опциональное исследование:

- local-only cache;
- shared Redis cache;
- replicated cache.

Можно измерять:

- hit ratio;
- latency;
- memory;
- network;
- recovery после отказа вычислительный узел.

---

# 10. Partitioning

Использовать больше логические разделы, чем вычислительный узел.

Например:

```text
128 логические разделы
4 вычислительный узел
```

Ownership:

```text
N1 -> P0..P31
N2 -> P32..P63
N3 -> P64..P95
N4 -> P96..P127
```

После scale-out:

```text
6 вычислительный узел

N1 -> subset
N2 -> subset
N3 -> subset
N4 -> subset
N5 -> subset
N6 -> subset
```

Важно:

**logical data partitions не обязаны быть 1:1 с Kafka partitions.**

Kafka отвечает за transport/parallel processing, а логические разделы — за data ownership/routing.

Например:

```text
128 logical data partitions
32 Kafka partitions
```

Для MVP допустима более простая конфигурация, но концептуально эти сущности должны оставаться раздельными.

---

# 11. Rebalancing

Rebalancing — потенциально одна из самых важных конструкторских частей.

Пример:

```text
Before:

N1: P1 P2 P3 P4
N2: P5 P6 P7 P8
N3: P9 P10 P11 P12
```

Scale-out:

```text
N4 starts
       |
       v
wait for READY
       |
       v
partition reassignment
```

После:

```text
N1: P1 P2 P3
N2: P5 P6 P7
N3: P9 P10
N4: P4 P8 P11 P12
```

Предпочтительная state machine:

```text
OWNED
  |
  v
TRANSFERRING
  |
  v
OWNED_BY_NEW_OWNER
```

Нужно продумать:

- ownership;
- epoch/generation;
- migration;
- acknowledgement;
- draining;
- stale owner;
- failure during migration;
- recovery;
- duplicate operations;
- lost operations;
- consistency.

---

# 12. Scale-out / Scale-in lifecycle

## Scale-out

1. Control Plane определяет `ScaleOut`.
2. Desired replicas увеличивается.
3. Kubernetes создаёт новые Pods вычислительных узлов.
4. Control Plane ждёт readiness.
5. Новый вычислительный узел регистрируется.
6. Partition Manager рассчитывает новый assignment.
7. Rebalance.
8. Старые вычислительный узел освобождают переданные partitions.
9. Assignment становится активным.
10. SystemState возвращается к нормальному.

Критически важно:

**не передавать partition новому вычислительный узел до его readiness.**

## Scale-in

1. Control Plane принимает `ScaleIn`.
2. Выбирается вычислительный узел для удаления.
3. Его partitions назначаются другим вычислительный узел.
4. вычислительный узел переходит в draining.
5. Новые запросы на его partitions не принимаются.
6. Завершаются текущие операции.
7. Ownership подтверждается новым вычислительный узел.
8. Kubernetes уменьшает replicas.
9. Состояние стабилизируется.

---

# 13. Control Plane

Control Plane — центральный объект исследования.

Логическая структура:

```text
                Metrics
                   |
                   v
          +------------------+
          | State Aggregator |
          +--------+---------+
                   |
                   v
          +------------------+
          | Workload Analyzer|
          +--------+---------+
                   |
                   v
          +------------------+
          | Bottleneck       |
          | Detector         |
          +--------+---------+
                   |
                   v
          +------------------+
          | Scaling /        |
          | Rebalance Policy |
          +--------+---------+
                   |
                   v
          +------------------+
          | Desired State    |
          +--------+---------+
                   |
                   v
          +------------------+
          | Reconciler       |
          +--------+---------+
                   |
                   v
              Kubernetes
```

Возможная типизированная модель:

```scala
sealed trait SystemState

case object Healthy extends SystemState
case object QueueBound extends SystemState
case object DatabaseBound extends SystemState
case object HotPartition extends SystemState
case object Overprovisioned extends SystemState

sealed trait ScalingDecision

case object ScaleOut extends ScalingDecision
case object ScaleIn extends ScalingDecision
case object Maintain extends ScalingDecision
case object Rebalance extends ScalingDecision
case object Throttle extends ScalingDecision
```

Функциональное ядро:

```scala
def decide(state: SystemState): ScalingDecision
```

Это не обязательная финальная реализация, а архитектурная модель.

---

# 14. Kubernetes и Docker Compose

## Kubernetes

Kubernetes отвечает за фактическое создание/удаление Pods.

Control Plane **не должен сам запускать контейнеры**.

Схема:

```text
Control Plane
      |
      | desired replicas = 5
      v
Kubernetes API
      |
      v
Deployment
      |
      v
Pods вычислительных узлов
```

Рекомендуется использовать:

- Deployment для вычислительный узел;
- Kubernetes API;
- readiness probes;
- service account/RBAC;
- metrics.

Custom Kubernetes Operator/CRD **не является обязательным**. Его не следует делать ради сложности, если это не понадобится.

## Docker Compose

Docker Compose — прежде всего development/MVP environment.

Можно использовать для:

- Kafka;
- PostgreSQL;
- Redis;
- Prometheus;
- Grafana;
- Control Plane;
- нескольких вычислительный узел.

Но основное исследование autoscaling лучше проводить в Kubernetes.

Не строить собственный полноценный Compose orchestrator через shell-команды.

Можно абстрагировать:

```text
trait Orchestrator {
    scale(service, replicas)
    currentReplicas(service)
}
```

и иметь Kubernetes implementation.

---

# 15. Desired state / Actual state

Control Plane должен мыслить не просто командами, а состояниями.

```text
Desired State:
replicas = 5
assignment = X

Actual State:
replicas = 3
assignment = Y

            |
            v

        Reconciler

            |
            v

Actual -> Desired
```

Это делает поведение контроллера устойчивым к:

- задержкам Kubernetes;
- сбоям;
- повторным командам;
- race conditions;
- частично выполненным операциям.

---

# 16. Kafka

Kafka используется как:

- буфер burst-нагрузки;
- средство decoupling;
- транспорт событий;
- механизм горизонтального parallel processing;
- replay/recovery;
- источник lag metrics.

Не говорить, что Kafka «убирает bottleneck».

Корректнее:

> Kafka буферизует входящую нагрузку и отделяет producer/consumer throughput, однако downstream storage всё равно может стать bottleneck.

Можно использовать Kafka для событий изменения ownership:

```text
PartitionAssignmentChanged(
    partition,
    oldOwner,
    newOwner,
    epoch
)
```

Однако Kafka consumer groups не должны полностью заменять собственную модель логический раздел ownership, если управление partitioning является частью исследования.

---

# 17. Query API / DSL

Не делать полноценный SQL engine.

Но возможен typed restricted query DSL.

Например:

```scala
storage
  .query[User]
  .where(_.age > 18)
  .where(_.country === "FI")
  .limit(100)
  .execute()
```

DSL может поддерживать:

- point lookup;
- batch lookup;
- range query;
- filter;
- projection;
- limit.

AST:

```text
Query
 |
 +-- Entity
 +-- Filters
 +-- Projection
 +-- Limit
```

Routing analysis:

```text
Query
  |
  v
Query classification
  |
  +--> partition key known
  |       |
  |       v
  |    Single вычислительный узел
  |
  +--> partition key unknown
          |
          v
      Distributed query
```

Distributed query может быть extension:

```text
N1 -> partial result
N2 -> partial result
N3 -> partial result
          |
          v
       merge
```

Полноценные:

- JOIN;
- GROUP BY;
- subqueries;
- window functions;
- SQL parser;
- optimizer

не входят в MVP.

DSL не является основной научной новизной.

---

# 18. Go + Scala

Использование двух языков должно иметь архитектурное обоснование.

## Go — Data Plane

Использовать для вычислительные узлы:

- HTTP/gRPC;
- Kafka;
- Redis;
- PostgreSQL;
- concurrency;
- deployment;
- benchmarks.

Причины:

- простой runtime;
- высокая производительность;
- удобная concurrency model;
- удобные clients;
- удобное контейнерное deployment.

## Scala — Control Plane

Использовать для:

- state model;
- scaling policy;
- typed ADTs;
- decision logic;
- query DSL;
- fs2/cats-effect или ZIO;
- функционального моделирования state machine.

Получается естественное разделение:

```text
Go
  =
Data Plane

Scala
  =
Control Plane
```

Не обязательно использовать оба языка, если в процессе исследования это создаёт неоправданную сложность. Архитектурная целесообразность важнее количества технологий.

---

# 19. MVP

## MVP должен содержать

- логические разделы;
- вычислительные узлы;
- routing;
- PostgreSQL;
- Kafka;
- local L1 cache;
- metrics;
- Control Plane;
- static scaling;
- basic adaptive scaling;
- Kubernetes deployment;
- benchmark;
- базовый rebalancing.

## Research-grade implementation

Добавить:

- bottleneck detection;
- multi-metric controller;
- hot partition detection;
- dynamic partition assignment;
- safe rebalancing;
- fault recovery;
- Redis L2;
- detailed observability.

## Stretch goals

Только если core system уже работает:

- replicated cache;
- predictive autoscaling;
- workload forecasting;
- ML-based prediction;
- distributed queries;
- adaptive consistency.

Не добавлять технологии только ради объёма.

Не использовать без необходимости:

- Cassandra;
- ClickHouse;
- Elasticsearch;
- Spark;
- Flink;
- ZooKeeper;
- Istio/service mesh;
- дополнительные брокеры.

---

# 20. Benchmark / Experimental Design

Минимальные baselines:

### Baseline 0

```text
Client -> PostgreSQL
```

### Baseline 1

```text
Client -> Static вычислительный узелs -> PostgreSQL
```

### Baseline 2

```text
Client -> вычислительный узелs -> PostgreSQL
              ^
              |
        Cвычислительный узел-based HPA
```

### Proposed

```text
Client -> вычислительный узелs -> Kafka/PostgreSQL/Cache
              ^
              |
       Adaptive Controller
       + dynamic partitioning
```

Опционально:

- Redis;
- Apache Ignite;
- другие существующие решения.

Но не ставить цель «победить Ignite во всём».

---

# 21. Workloads

Минимальный набор:

1. Uniform workload.
2. Bursty workload.
3. Zipfian/skewed workload.
4. Hot partition.
5. Moving hot partition.
6. вычислительный узел failure.
7. PostgreSQL slowdown.

---

# 22. Метрики

Основные:

- RPS / throughput;
- p50 latency;
- p95 latency;
- p99 latency;
- Cвычислительный узел;
- memory;
- Kafka lag;
- Kafka throughput;
- Redis hit ratio;
- PostgreSQL latency;
- PostgreSQL connections;
- database QPS;
- partition load;
- partition skew;
- number of вычислительный узел;
- scaling decision latency;
- scaling convergence time;
- rebalance duration;
- recovery time;
- resource efficiency.

При необходимости:

- network traffic;
- migration bytes;
- number of migrations;
- cache warm-up time.

---

# 23. План исследовательских экспериментов

## Experiment 1 — Partitioning

Сравнить варианты распределения.

Цель:

- migration overhead;
- rebalance time;
- scalability.

## Experiment 2 — Scaling

Сравнить:

- static;
- Cвычислительный узел-based;
- proposed multi-metric.

Workloads:

- stable;
- burst;
- gradual growth;
- sudden decrease.

## Experiment 3 — Hot partitions

Сравнить:

- static assignment;
- dynamic assignment.

Измерить tail latency и skew.

## Experiment 4 — Failure

Убить вычислительный узел.

Измерить:

- detection time;
- reassignment time;
- recovery time;
- latency degradation;
- cache warm-up.

## Experiment 5 — Storage bottleneck

Искусственно ограничить/замедлить PostgreSQL.

Проверить, не приводит ли предложенный controller к бессмысленному scale-out.

---

# 24. Структура ВКР

## Введение

- актуальность;
- проблема;
- объект;
- предмет;
- цель;
- задачи;
- методы;
- научная новизна;
- практическая значимость.

## 1. Исследование предметной области

- распределённые системы;
- distributed storage;
- Space-Based Architecture;
- вычислительные узлы;
- Kafka;
- caching;
- partitioning;
- autoscaling;
- существующие решения;
- анализ аналогов.

## 2. Исследовательская часть

### 2.1. RQ1 — partitioning/rebalancing

### 2.2. RQ2 — adaptive scaling

### 2.3. RQ3 — skew/hot partitions

### 2.4. Методика экспериментов

### 2.5. Результаты

### 2.6. Выводы и выбор архитектурных решений

Целевой объём исследовательской части: ~50 страниц.

## 3. Конструкторская часть

### 3.1. Архитектура системы
### 3.2. вычислительный узел
### 3.3. Data model
### 3.4. Partitioning
### 3.5. Rebalancing
### 3.6. Cache
### 3.7. Control Plane
### 3.8. Scaling controller
### 3.9. Kubernetes deployment
### 3.10. Failure/recovery
### 3.11. API/DSL

Целевой объём: ~40 страниц.

## 4. Разработка технологии

~10 страниц.

В этом документе подробно не рассматривается.

---

# 25. Две статьи

## Статья 1 — архитектура

Рабочее название:

> **Архитектура адаптивной распределённой системы хранения данных на основе динамически масштабируемых вычислительные узлы**

7–10 страниц.

Суть:

1. проблема;
2. существующие подходы;
3. Space-Based Architecture;
4. proposed architecture;
5. вычислительные узлы;
6. partitioning;
7. Control Plane;
8. scaling model;
9. MVP;
10. предварительные результаты.

## Статья 2 — экспериментальная

Рабочее название:

> **Исследование адаптивного масштабирования распределённой системы хранения при неравномерной нагрузке**

7–10 страниц.

Суть:

1. постановка;
2. workload;
3. baselines;
4. Cвычислительный узел autoscaling;
5. proposed controller;
6. uniform;
7. burst;
8. Zipf;
9. hot partition;
10. результаты.

Статьи должны быть частями одной исследовательской линии, а не двумя независимыми проектами.

---

# 26. План плакатов А1

Планируется 10 листов.

1. **Постановка задачи**
   - проблема;
   - цель;
   - задачи;
   - исследовательские вопросы.

2. **Анализ аналогов**
   - PostgreSQL;
   - Redis;
   - Ignite;
   - Hazelcast;
   - Space-Based Architecture;
   - proposed approach.

3. **Общая архитектура системы**

4. **Структурная схема вычислительный узел**

5. **Информационная/БД схема**
   - PostgreSQL;
   - metadata;
   - entities;
   - indexes.

6. **Partitioning и rebalancing**

7. **Control Plane**

8. **Алгоритм адаптивного масштабирования**
   - SystemState;
   - state machine;
   - decision function.

9. **Deployment diagram**
   - Kubernetes;
   - Pods;
   - Kafka;
   - Redis;
   - PostgreSQL;
   - Control Plane.

10. **Экспериментальные результаты**
    - RPS;
    - p95/p99;
    - Cвычислительный узел;
    - Kafka lag;
    - scaling timeline;
    - recovery;
    - comparison.

---

# 27. План разработки

## Сентябрь 2026

- окончательно зафиксировать тему;
- формализовать объект/предмет/цель;
- сформулировать RQ;
- literature review;
- анализ аналогов;
- архитектурный дизайн;
- репозиторий;
- MVP skeleton.

## Октябрь 2026

- вычислительный узел;
- PostgreSQL;
- Kafka;
- routing;
- логические разделы;
- local cache;
- базовые metrics;
- Kubernetes;
- первые benchmarks;
- статья №1;
- статья №2 — первый вариант/основные эксперименты.

## Ноябрь 2026

- Control Plane;
- multi-metric state;
- bottleneck detector;
- scaling policy;
- Kubernetes reconciliation;
- basic autoscaling;
- rebalancing.

## Декабрь 2026

- полноценные benchmarks;
- hot partitions;
- skew;
- burst;
- failure;
- cache experiments;
- сбор графиков;
- финализация исследовательских результатов.

## Январь 2027

- написание основной ВКР;
- оформление исследовательской части;
- оформление конструкторской части;
- финальные эксперименты;
- диаграммы;
- таблицы.

## Февраль 2027

- финальная версия;
- проверка результатов;
- исправления;
- плакаты;
- подготовка защиты;
- резерв времени.

## 01.03.2027

- готовая ВКРМ.

---

# 28. Принципы работы AI/Codex-агента

AI-агент должен работать **итеративно**, а не пытаться сразу написать весь проект.

Правильный цикл:

```text
Architecture decision
       |
       v
Small implementation task
       |
       v
Tests
       |
       v
Run
       |
       v
Benchmark / validation
       |
       v
Review
       |
       v
Next task
```

Перед каждой крупной реализацией агент должен понимать:

- зачем компонент существует;
- какой исследовательский вопрос он поддерживает;
- какие интерфейсы нужны;
- как его проверить;
- какие метрики получить.

Не следует:

- переписывать всю систему при каждой итерации;
- добавлять технологии без необходимости;
- делать feature ради количества кода;
- выдавать неподтверждённые benchmark results;
- считать предположительные результаты фактическими;
- заявлять научную новизну без экспериментального подтверждения.

---

# 29. Что является обязательным, а что нет

## Обязательно

- вычислительные узлы;
- логический разделing;
- Control Plane;
- adaptive scaling;
- baseline comparison;
- metrics;
- benchmarks;
- rebalancing;
- Kubernetes;
- failure scenario;
- научные выводы.

## Желательно

- Redis L2;
- typed DSL;
- sophisticated bottleneck states;
- cache experiments.

## Только при наличии времени

- predictive scaling;
- ML;
- replicated cache;
- distributed query execution.

---

# 30. Главный scope boundary

Не превращать проект в полноценную СУБД.

Мы **не строим**:

- SQL parser;
- полноценный query optimizer;
- transaction engine;
- WAL;
- собственный storage engine;
- собственный consensus algorithm;
- собственный message broker;
- собственный container orchestrator.

Мы строим **distributed processing/storage architecture and adaptive control mechanism**, используя PostgreSQL, Kafka, Redis и Kubernetes как инфраструктурные компоненты.

---

# 31. Критерий успешности ВКР

Работа считается успешной, если выполнены все условия:

1. Есть формальная постановка проблемы.
2. Есть анализ существующих решений.
3. Есть 3 исследовательских вопроса.
4. Для каждого есть методика эксперимента.
5. Есть результаты, а не только архитектурные схемы.
6. Есть обоснованный выбор архитектурных решений.
7. Реализован работающий MVP.
8. Реализован adaptive controller.
9. Реализован dynamic partition assignment/rebalancing.
10. Есть сравнение со static/Cвычислительный узел-based baseline.
11. Есть сценарии skew/burst/failure.
12. Есть воспроизводимые benchmark results.
13. Есть две статьи.
14. Есть ~10 содержательных А1.
15. Объём ВКР можно естественно довести до ~100 страниц.

---

# 32. Следующая техническая задача

После фиксации общего контекста следующая наиболее важная задача:

> **Формализовать жизненный цикл логический раздел при переходе `3 вычислительный узел → 5 вычислительный узел → 2 вычислительный узел`, включая ownership, epoch, migration, cache invalidation/warming, Kafka offsets, duplicate/lost operations, graceful draining и failure during rebalance.**

Это следует сделать до реализации сложного autoscaling, потому что именно здесь находится значительная часть реальной распределённой системной сложности.

После этого можно переходить к формализации:

```text
SystemState
        |
        v
ScalingDecision
        |
        v
DesiredClusterState
        |
        v
PartitionAssignment
        |
        v
Reconciliation
```

и уже затем реализовывать MVP поэтапно.


---

# 29. Актуальный технологический стек и порядок реализации

Этот раздел является **каноническим** для порядка реализации. Более ранние разделы документа описывают архитектурные компоненты и исследовательские идеи, но не должны интерпретироваться как требование реализовывать Kafka/Redis раньше Control Plane и публикаций.

## 29.1. Языки и ответственность компонентов

### Go — вычислительный слой

На Go реализуются:

- вычислительные узлы;
- API обработки запросов;
- Router (если выделяется в отдельный сервис);
- Partition Manager;
- PostgreSQL repository;
- локальный L1 cache;
- Kafka producer/consumer после раннего публикационного этапа;
- Redis client после раннего публикационного этапа;
- метрики data plane.

Причины выбора:

- удобная модель конкурентности;
- зрелые клиенты PostgreSQL/Kafka/Redis;
- простое контейнерное развёртывание;
- низкие накладные расходы;
- удобство нагрузочного тестирования.

### Scala — Control Plane

Предпочтительно Scala 3 + ZIO.

На Scala реализуются:

- State Aggregator;
- SystemState;
- Bottleneck Detector;
- Scaling Policy;
- Rebalancing Policy;
- DesiredClusterState;
- PartitionAssignment calculation;
- Reconciliation Loop;
- Kubernetes adapter.

Функциональное ядро:

```scala
def decide(state: SystemState): ScalingDecision
```

Если разделение на два языка начнёт создавать непропорциональную сложность, допустимо унифицировать реализацию. Научный результат не должен зависеть от количества используемых языков.

## 29.2. Хранилища и транспорт

### PostgreSQL

Роль: authoritative persistent storage.

Используется с самого первого рабочего прототипа.

### Kafka

Роль:

- асинхронный транспорт;
- буферизация;
- decoupling;
- обработка событий/операций;
- экспериментальная метрика lag.

Kafka **не является механизмом устранения bottleneck** и не должна добавляться до того, как готов ранний фундамент и публикационный путь.

### Redis

Роль: опциональный shared L2 cache.

Иерархия:

```text
L1 local memory
    ↓ miss
L2 Redis
    ↓ miss
PostgreSQL
```

MVP и ранние статьи могут работать без Redis.

## 29.3. Инфраструктура

### Docker Compose

Используется для:

- локальной разработки;
- запуска PostgreSQL;
- позднее Kafka/Redis;
- быстрых интеграционных тестов.

### Kubernetes

Используется для:

- запуска нескольких экземпляров вычислительных узлов;
- изменения replica count;
- readiness/liveness;
- resource requests/limits;
- CPU/HPA baseline;
- исполнения DesiredClusterState.

Control Plane не запускает контейнеры напрямую.

### Prometheus + Grafana

Используются для:

- CPU/RAM;
- RPS;
- p50/p95/p99;
- active requests;
- queue size;
- PostgreSQL latency;
- Kafka lag после появления Kafka;
- partition load/skew;
- визуализации временных рядов.

## 29.4. Benchmark tooling

Нужен отдельный workload generator / benchmark runner.

Он должен поддерживать:

```text
constant
ramp
burst
spike
periodic
uniform
Zipf
hot partition
moving hot partition
```

Каждый запуск должен сохранять:

- конфигурацию;
- версию реализации;
- параметры окружения;
- raw metrics;
- агрегированные результаты;
- графики.

---

# 30. Актуальная последовательность разработки

## Этап A — фундамент, 0–25%

### A0. Formal Model

Реализовать/зафиксировать:

```text
Record
NodeId
PartitionId
PartitionAssignment
Epoch
System invariants
Routing model
```

### A1. Go Node + PostgreSQL

```text
Client → Go Node → PostgreSQL
```

Минимум:

- GET;
- PUT;
- DELETE;
- connection pool;
- timeout/cancellation;
- graceful shutdown;
- baseline benchmark.

### A2. Logical Partitions

```text
PartitionId = hash(key) mod P
```

Количество разделов фиксировано и не зависит от node count.

### A3. Multi-node + Router

```text
key
 ↓
PartitionId
 ↓
PartitionAssignment
 ↓
NodeId
```

### A4. Assignment + Epoch

Защита от stale topology/stale owner.

### A5. Partitioning Research Harness

Сравнить:

```text
hash(key) % nodeCount
consistent hashing
fixed logical partitions + dynamic assignment
```

После A5 готовится **статья №1**:

> «Исследование методов логического распределения данных в динамически масштабируемой распределённой системе»

---

## Этап B — ранний Control Plane и масштабирование, 25–45%

### B1. Observability

Сначала сделать систему измеряемой:

```text
RPS
p50/p95/p99
CPU
RAM
active requests
queue size
PostgreSQL latency
partition load
```

### B2. Kubernetes

Перенести вычислительные узлы в Kubernetes.

### B3. Scala/ZIO Control Plane v1

```text
Metrics
 ↓
State Aggregator
 ↓
SystemState
 ↓
Decision
```

### B4. Baselines

Реализовать:

```text
Static replicas
CPU-based / HPA scaling
```

### B5. Early Multi-Metric Controller

Минимальный вход:

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

### B6. Early Scaling Benchmarks

Нагрузки:

```text
constant
ramp
burst
spike
periodic
```

Сравнить:

```text
Static
CPU/HPA
Multi-Metric
```

После B6 готовится **статья №2**:

> «Исследование многокритериального масштабирования вычислительных узлов распределённой системы»

---

## Этап C — расширение Data Plane, 45–55%

### C1. Kafka

Добавить producer/consumer, topics, consumer groups, offsets, batching.

### C2. Idempotency

Добавить operation_id, deduplication и корректную at-least-once обработку.

### C3. L1 Cache

Локальный cache-aside cache вычислительного узла.

### C4. Redis L2

Опциональный shared cache.

Важно: Kafka и Redis добавляются здесь, потому что они нужны полной системе и поздним экспериментам, но не должны задерживать две статьи.

---

## Этап D — безопасная динамическая топология, 55–70%

### D1. Rebalancing

Рассчитывать новую PartitionAssignment при изменении topology.

### D2. Partition State Machine

```text
OWNED
 ↓
PREPARING
 ↓
TRANSFERRING
 ↓
ACTIVATING
 ↓
OWNED
```

### D3. Graceful Draining

Полностью реализовать:

```text
3 nodes → 5 nodes
5 nodes → 2 nodes
```

### D4. Kafka During Migration

Формализовать ownership обработки, offsets, duplicates и ordering.

### D5. Cache Migration

MVP:

```text
old owner invalidates L1
new owner starts cold
new owner warms from Redis/PostgreSQL
```

### D6. Failure Recovery

Сценарии:

- owner crash;
- target crash during migration;
- Control Plane restart;
- duplicate reconciliation;
- stale owner.

---

## Этап E — полный Adaptive Controller, 70–85%

### E1. Hot Partition Detection

Использовать partition-level metrics и skew.

### E2. Load-Aware Placement

Решать, какой раздел и на какой узел переносить с учётом migration cost.

### E3. Bottleneck Classification

Состояния:

```text
Healthy
ComputeBound
QueueBound
DatabaseBound
HotPartition
Overprovisioned
```

### E4. Adaptive Controller

Решения:

```text
Maintain
ScaleOut
ScaleIn
Rebalance
ScaleOut + Rebalance
Throttle (опционально)
```

### E5. Oscillation Protection

Добавить:

- hysteresis;
- cooldown;
- stabilization windows;
- EWMA.

### E6. Reconciliation

Канонический pipeline:

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
    ↓
Kubernetes
```

---

## Этап F — финальные исследования, 85–100%

### F1. Reproducible Benchmark Framework

Формализовать экспериментальную методологию.

### F2. RQ1 Extended

Расширить статью №1 реальными migration/rebalancing experiments.

### F3. RQ2 Extended

Сравнить:

```text
Static
CPU/HPA
Early Multi-Metric
Full Adaptive
```

### F4. RQ3 Hot Partitions

Сравнить adaptive rebalancing с системой без него на Uniform/Zipf/Hot/Moving-Hot workloads.

### F5. Fault Experiments

Проверить recovery и convergence при отказах.

---

# 31. Актуальная публикационная стратегия

## Статья №1 — ~20–25%

**Тема:**

> «Исследование методов логического распределения данных в динамически масштабируемой распределённой системе»

Не требует Kafka, Redis, Kubernetes или полноценного rebalancing.

Основные результаты:

- сравнение partitioning algorithms;
- migration ratio;
- load distribution;
- стоимость topology change;
- ранний multi-node prototype.

## Статья №2 — ~40–45%

**Тема:**

> «Исследование многокритериального масштабирования вычислительных узлов распределённой системы»

Не требует safe partition migration, hot-partition handling или fault recovery.

Основные результаты:

- Static baseline;
- CPU/HPA baseline;
- ранний multi-metric controller;
- dynamic workload profiles;
- reaction/stabilization time;
- p95/p99;
- resource efficiency.

## Полная ВКР

Объединяет результаты статей и добавляет:

- Kafka;
- L1/L2 cache;
- safe rebalancing;
- ownership/epoch;
- failure recovery;
- hot partition detection;
- load-aware placement;
- bottleneck-aware adaptive controller;
- reconciliation;
- расширенные RQ1/RQ2/RQ3.

---

# 32. Что не входит в обязательный scope

Не добавлять без исследовательской необходимости:

- Cassandra;
- ClickHouse;
- Elasticsearch;
- Spark;
- Flink;
- ZooKeeper;
- Istio/service mesh;
- собственный consensus protocol;
- собственный message broker;
- собственный storage engine;
- полный SQL parser/query optimizer;
- ML/predictive autoscaling до завершения rule-based adaptive controller.

Принцип:

> Новая технология добавляется только тогда, когда она необходима для конкретного инварианта, исследовательского вопроса или измеримого эксперимента.

---

# 33. Ближайший технический entry point

До Kafka, Redis и сложного Control Plane необходимо закрыть:

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

Затем реализовать:

```text
Go Node + PostgreSQL
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

Это является текущим приоритетным путём реализации.
