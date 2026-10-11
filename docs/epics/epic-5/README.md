# Эпик 5 — Kubernetes static baseline и placement contract

## Цель

Перенести существующий стенд в локальный Kubernetes и получить воспроизводимую статическую baseline-конфигурацию для будущего сравнения способов масштабирования. Эпик проверяет развёртывание, стабильную идентичность Processing Unit, согласованность assignment и наблюдаемость в Kubernetes, но намеренно не исполняет advisory-решения Control Plane.

Эпик продолжает завершённый [наблюдающий Control Plane v1](../epic-4/README.md) и закрывает Kubernetes-фундамент B2. CPU/HPA baseline, multi-metric actuator и автоматическая перестройка assignment остаются отдельными последующими инкрементами: до них необходимо определить жизненный цикл membership и безопасный scale-in.

## Целевой поток

```text
kind cluster
    ↓
статические Kubernetes workload и service contracts
    ↓
стабильные Processing Unit identities
    ↓
bootstrap authoritative assignment
    ↓
Router + Processing Units + PostgreSQL
    ↓
Prometheus → Control Plane → advisory decision
    ↓
Grafana overview и validation evidence
```

## Scope

### 5.1. Воспроизводимый локальный кластер

- использовать `kind` с закреплённой версией node image;
- добавить одну документированную команду подготовки свежего кластера, загрузки локальных application images и применения manifests;
- хранить Kubernetes-конфигурацию как native manifests с Kustomize, без генерации инфраструктурной схемы из Go или Scala;
- добавить namespace, ConfigMap, шаблон Secret без реальных credentials, ServiceAccount и только минимально необходимый RBAC;
- сохранить Docker Compose как быстрый локальный и regression-стенд.

### 5.2. Статическое размещение компонентов

- развернуть PostgreSQL с persistent volume, Control Plane, Router, Processing Units, Prometheus, Grafana и отдельный load-generator Job;
- задать readiness/liveness/startup probes по фактическим operational endpoints;
- настроить graceful termination и достаточный termination grace period;
- откалибровать requests/limits экспериментально, отдельно проверив headroom Prometheus, Grafana и генератора нагрузки;
- оставить число Processing Units статическим на всём протяжении эпика.

### 5.3. Identity и assignment contract

- обеспечить стабильные адресуемые идентичности Processing Units, предпочтительно через StatefulSet и headless Service;
- создавать начальный authoritative assignment явным bootstrap Job или вызовом Assignment API, а не SQL-миграцией;
- подтвердить, что Router получает тот же epoch/node set, а Processing Unit продолжает атомарно проверять ownership и epoch вместе с SQL-операцией;
- не считать Kubernetes replica count источником истины для assignment;
- описать контракт между Kubernetes pod identity, application node ID, service discovery name и assignment node ID.

### 5.4. Kubernetes-наблюдаемость

- собирать CPU и память Processing Units из Kubernetes/cAdvisor-метрик;
- нормализовать Compose- и Kubernetes-сигналы до одинаковых канонических per-node series, предпочтительно Prometheus recording rules;
- не менять доменную policy Control Plane в зависимости от среды развёртывания;
- сохранить семантику missing/stale metrics: отсутствие данных не отображается как ноль и запрещает scaling decision;
- расширить обзорный Grafana dashboard показателями desired/ready/available replicas, числом нод authoritative assignment и advisory recommended replicas.

### 5.5. Проверка baseline

- прогнать `constant`, `ramp` и `burst` workload через Router;
- подтвердить CRUD, CAS assignment, Router refresh и PostgreSQL fencing;
- проверить рестарт Router, одной Processing Unit, Control Plane, Prometheus и PostgreSQL в пределах заявленной модели восстановления;
- зафиксировать фактические requests/limits, окна наблюдения, нагрузку, состояние pod и скриншоты dashboard;
- выполнить Compose regression после Kubernetes-проверок.

## Критерии приёмки

Эпик готов, когда:

1. Новый локальный `kind` cluster поднимается из документированной команды без ручного редактирования manifests.
2. Все application images загружены локально, а workload переходят в Ready с настроенными probes.
3. Работают три статические Processing Units со стабильными node IDs и адресами.
4. Authoritative assignment содержит ровно эти ноды и согласован с Router snapshot; stale epoch/owner по-прежнему отклоняется Processing Unit.
5. Control Plane строит полный `SystemState`, включая Kubernetes CPU/RAM, и остаётся только advisory.
6. Grafana показывает deployment state, assignment node count, recommended replica reference и policy thresholds без подмены missing data нулём.
7. `constant`, `ramp` и `burst`, а также выбранные restart/failure сценарии подтверждены фактическими артефактами.
8. Контейнерные и Compose regression-проверки текущих API проходят после изменений.
9. Validation report отделяет выполненные проверки от ограничений и будущего CPU/HPA эксперимента.

## Вне scope

В этом эпике не реализуются:

- HPA и любое автоматическое изменение replica count;
- Kubernetes API client или actuator внутри Control Plane;
- автоматическое обнаружение topology и публикация assignment;
- allocator на Scala, rebalancing, draining и физическая миграция данных;
- безопасный динамический scale-in;
- Kubernetes Operator, Kafka, Redis и полноформатный сравнительный benchmark B6.

`ScalingDecision.recommendedReplicas` остаётся наблюдаемым reference value: оно показывает, какой размер пула предлагает policy при текущем состоянии, но не является desired state Kubernetes и ничего не меняет.

## Рекомендуемое продолжение

После статического baseline отдельным эпиком определить membership/reconciliation contract и только затем сравнить CPU/HPA baseline с исполняемой multi-metric policy. Для scale-out требуется ввод готовой ноды в assignment; для scale-in — выбор удаляемой ноды, draining и атомарная публикация нового epoch. Простое связывание текущего reference value с replicas нарушило бы уже реализованные ownership-инварианты.

## Старт следующей агентской сессии

1. Создать ветку `epic/5-kubernetes-baseline` от принятого состояния Epic 4.
2. Проверить локальные версии Docker, `kind` и `kubectl`, не перезапуская WSL или Docker Engine.
3. Сначала зафиксировать placement/identity contract и структуру manifests, затем добавлять workload.
4. Сохранять промежуточные решения в этом документе и `AGENTS.md`, если появляется устойчивое правило для всего репозитория.
5. Создать `changelog.md` и `validation.md` по мере реализации; результаты и скриншоты считать принятыми только после фактического прогона.
