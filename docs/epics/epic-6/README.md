# Эпик 6 — безопасное Kubernetes actuation и reconciliation топологии

## Статус и цель

Эпик реализуется в ветке `epic/6-kubernetes-actuation` поверх завершённого [эпика 5](../epic-5/README.md). Он связывает подтверждённое решение multi-metric policy с реальным изменением числа Processing Unit в Kubernetes, не передавая Kubernetes-понятия в domain и service слои. Основной runtime-срез и локальный цикл `3 → 4 → 3` подтверждены в [validation report](validation.md).

Целевой поток:

```text
SystemState
    ↓
ScalingPolicy → confirmed ScalingDecision
    ↓
TopologyReconciler → DesiredClusterState
    ├── ProcessingUnitFleet port → Kubernetes adapter → StatefulSet scale API
    ├── PartitionAllocator → новый полный assignment
    ├── AssignmentService → PostgreSQL CAS + fencing
    └── RouterTopologyObserver → подтверждение установленного epoch
```

Эпик реализует первый безопасный actuator для уже существующего decision-ядра B5 и создаёт membership/reconciliation contract, необходимый перед CPU/HPA baseline B4. Сравнительный benchmark B6 остаётся отдельным следующим этапом.

## Исходные гарантии

- active assignment в PostgreSQL, а не Kubernetes replica count, остаётся источником истины topology;
- Processing Unit не хранит локальное авторитетное состояние: документы находятся в общей PostgreSQL;
- Router направляет запрос конкретному владельцу logical partition;
- публикация assignment и CRUD используют общий PostgreSQL lock, поэтому stale owner не может закоммитить операцию после смены epoch;
- StatefulSet предоставляет стабильные `processing-unit-N` и индивидуальные headless DNS-адреса;
- scaling policy остаётся чистой функцией и не вызывает инфраструктуру;
- один подтверждённый decision изменяет topology не более чем на настроенный step, первоначально `±1`.

## Инварианты reconciliation

1. В каждый момент выполняется не более одной topology operation.
2. Операция принимается только для текущего assignment epoch и текущего authoritative node count.
3. Новая нода попадает в assignment только после Kubernetes readiness и прикладной readiness Processing Unit.
4. Удаляемая нода исключается из assignment до уменьшения StatefulSet.
5. Scale-in не удаляет Pod, пока Router не подтвердил новый epoch и отсутствие удаляемой ноды в своей карте.
6. Каждый новый assignment полон, содержит ровно одного владельца каждого partition и публикуется через CAS.
7. Рестарт Control Plane не создаёт вторую операцию и не оставляет систему в неопределяемой фазе.
8. Ошибка Kubernetes, Router или assignment publication останавливает дальнейший переход, но не отменяет уже установленный fence.
9. Compose по умолчанию остаётся advisory и не получает Kubernetes-зависимостей.
10. Domain и service слои не содержат Pod, StatefulSet, namespace, Kubernetes conditions, resource version или Kubernetes client types.

## Feature 6.1 — desired state и состояние операции

Добавить отдельный feature slice `reconciliation`, не смешивая его с `scaling`, `systemstate` или `assignment`:

```text
control-plane/src/main/scala/storage/controlplane/reconciliation/
├── domain/
├── service/
├── repository/postgres/
├── client/kubernetes/
├── client/router/http/
└── transport/{dto,http}/
```

Минимальная доменная модель:

```text
DesiredClusterState
├── replicas
├── basedOnAssignmentEpoch
└── requestedAt

ScalingOperation
├── id
├── direction
├── sourceAssignmentEpoch
├── sourceReplicas
├── targetReplicas
├── phase
├── startedAt
├── updatedAt
└── failure
```

Фазы должны описывать бизнесовый переход, а не названия Kubernetes calls: `Planning`, `Provisioning`, `WaitingForMembers`, `PublishingAssignment`, `WaitingForRouters`, `RemovingMembers`, `Completed`, `Failed`.

Активная операция сохраняется в PostgreSQL. Repository обеспечивает compare-and-set для фаз и единственность незавершённой операции. После рестарта reconciler читает операцию и фактические состояния assignment/fleet, затем продолжает идемпотентно. История не должна использоваться как источник текущего assignment.

Policy confirmation и cooldown остаются её собственным состоянием. Reconciler не принимает новый actionable decision, пока assignment, fleet и предыдущая операция не сошлись.

## Feature 6.2 — нейтральные порты инфраструктуры

Порты объявляются рядом с `TopologyReconciler` и используют только собственные доменные типы:

```scala
trait ProcessingUnitFleet {
  def observe: IO[FleetError, FleetSnapshot]
  def resize(target: ReplicaCount): IO[FleetError, Unit]
}

trait PartitionAllocator {
  def allocate(
      current: AssignmentSnapshot,
      targetMembers: Set[ProcessingUnitMember]
  ): Either[AllocationError, AssignmentSnapshot]
}

trait RouterTopologyObserver {
  def current: IO[TopologyObservationError, RouterTopologyState]
}
```

`FleetSnapshot` содержит desired/observed/ready count и готовых членов с application `nodeId` и URL. Он не раскрывает Kubernetes DTO. Для scale-in адаптер обязан подтвердить, что изменение replica count удалит именно исключённые из target assignment экземпляры; несовпадение является ошибкой planning, а не поводом удалить произвольную ноду.

## Feature 6.3 — runtime allocator

Реализовать в Scala production-вариант fixed logical partitions + explicit assignment. Исследовательский Go-модуль не импортируется приложением.

Allocator должен:

- сохранять максимально возможное число прежних владельцев;
- распределять partitions с разницей не более одной;
- возвращать полный детерминированный snapshot;
- не зависеть от Kubernetes identity или readiness types;
- использовать cross-language golden cases `3 → 4`, `3 → 5`, `5 → 3`, `5 → 10` из эпика 2;
- сохранять `partition_count` и повышать epoch ровно на один при последующей публикации.

Переназначение является логическим. Физического переноса строк нет, поскольку все Processing Unit используют общую авторитетную PostgreSQL.

## Feature 6.4 — Kubernetes Processing Unit fleet adapter

Первый адаптер управляет существующим StatefulSet `processing-unit`:

- читает StatefulSet и принадлежащие ему Pods;
- получает точный набор Ready members, а не только агрегат `readyReplicas`;
- изменяет `.spec.replicas` через subresource `statefulsets/scale`;
- учитывает `.spec.ordinals.start: 1` и фактический порядок удаления ordinal;
- сопоставляет Pod name, application node ID и headless DNS URL;
- классифицирует timeout, conflict, forbidden, invalid state и unavailable;
- применяет ограниченные timeout/backoff и не выполняет бесконечный блокирующий вызов.

Только Control Plane получает отдельный ServiceAccount. Минимальный namespace RBAC разрешает чтение нужного StatefulSet и Pods и изменение `statefulsets/scale`. Router, Processing Unit, bootstrap, Grafana и остальные application workloads остаются без API token. Cluster-scoped доступ actuator не требуется.

Конкретный Kubernetes adapter выбирается только в composition root. Compose использует disabled/advisory implementation и сохраняет существующий запуск без Kubernetes configuration.

## Feature 6.5 — scale-out protocol

```text
confirmed ScaleOut for epoch N
→ создать operation с target R+1
→ увеличить StatefulSet до R+1
→ дождаться точного нового Ready member
→ построить assignment для R+1 members
→ опубликовать epoch N+1 через CAS
→ дождаться установки epoch N+1 в Router
→ дождаться свежего полного SystemState для нового assignment
→ завершить operation
```

Assignment не включает неготовый Pod. Readiness timeout не меняет assignment. Созданная, но не включённая в assignment лишняя реплика не принимает пользовательский трафик и может быть безопасно удалена при rollback/retry. CAS conflict заставляет перечитать состояние и перепланировать, но не публиковать snapshot от устаревшего epoch.

## Feature 6.6 — safe scale-in protocol

```text
confirmed ScaleIn for epoch N
→ определить точные StatefulSet members, которые будут удалены
→ построить assignment только для survivors
→ опубликовать epoch N+1 через CAS
→ дождаться установки epoch N+1 во всех Router
→ уменьшить StatefulSet
→ подтвердить исчезновение victims и convergence fleet
→ завершить operation
```

Router должен предоставить cluster-internal read-only topology status с установленными `epoch`, `partition_count` и node IDs. Readiness Router и Prometheus samples недостаточны для safety condition: они не доказывают, что удаляемая нода исчезла из локальной карты маршрутизации. Endpoint не публикуется через Ingress.

PostgreSQL fencing упорядочивает уже начатые операции и публикацию assignment. Запрос Router со старым snapshot после публикации может получить `409 wrong_assignment`, поэтому этот эпик гарантирует correctness, но не обещает zero-error draining. Управление in-flight запросами и устранение временного availability gap остаются D2–D3.

Если Router convergence не подтверждён, StatefulSet не уменьшается. Если Control Plane перезапущен после публикации нового assignment, дополнительный неавторитетный Pod безопасно остаётся запущенным до продолжения операции.

## Feature 6.7 — API, метрики и конфигурация

Добавить cluster-internal read-only endpoint:

```text
GET /control/reconciliation
```

Он возвращает active/last operation, phase, source/target replicas, assignment epoch, наблюдаемый fleet и последнюю классифицированную ошибку. HTTP DTO остаются отдельно от domain.

Добавить bounded-cardinality метрики:

- текущая phase reconciliation;
- число операций по фиксированным `direction` и `outcome`;
- длительность операции;
- authoritative node count, desired replicas и ready replicas;
- assignment/fleet drift;
- Kubernetes API failures по фиксированному классу;
- ожидание Router convergence;
- время последней успешной операции.

Node ID, Pod name, operation ID и произвольный текст ошибки не используются как metric labels.

Строгий Control Plane config получает секцию actuation с режимом `advisory | kubernetes`, именем target workload, timeout/backoff и reconciliation interval. Compose по умолчанию использует `advisory`; Helm явно включает `kubernetes`. Недоступность Kubernetes не ломает Assignment API и не снимает PostgreSQL fencing, но блокирует новые операции и переводит actuator status в degraded.

## Feature 6.8 — deployment и эксплуатационные границы

Helm chart добавляет отдельный Control Plane ServiceAccount, namespace Role и RoleBinding. Workload name, namespace и service DNS contract задаются инфраструктурной конфигурацией, но не scaling policy.

Bootstrap Job остаётся только механизмом initial install/upgrade существующего статического состояния. После epoch 2 runtime topology изменяет исключительно reconciler; bootstrap не корректирует dynamic assignment.

Выполнение actuator можно явно отключить без изменения policy, API наблюдения или assignment administration. Deployment в CI по-прежнему не выполняется; CI проверяет Scala tests и оба Helm render mode с RBAC.

## StatefulSet и будущий Deployment

В этом эпике Processing Unit остаётся StatefulSet. Он используется для стабильной identity, индивидуальной DNS-адресации и предсказуемого scale-in, а не для локального хранения данных. Порт `ProcessingUnitFleet` не должен зависеть от workload kind.

Решение о переходе на Deployment принимается на этапе D6 `Failure Recovery`, после D2 partition state machine и D3 graceful draining. Переход выполняется только если recovery-модель требует динамического membership с эфемерными Pod identity и даёт измеримую пользу.

Перед заменой должны существовать:

- обнаружение и регистрация готовых экземпляров без ordinal identity;
- безопасное исключение конкретной ноды до её удаления;
- восстановление assignment при исчезновении или смене Pod identity;
- адресация конкретного владельца без балансировки на произвольную реплику общего Service;
- проверенный failure recovery для исчезновения ноды во время topology operation.

Если эти требования не дают преимущества, StatefulSet остаётся допустимой реализацией и после D6.

## Вне scope

- прямой HPA на StatefulSet;
- CPU/HPA comparative baseline и B6 benchmark;
- Kubernetes Operator, CRD и admission webhook;
- physical partition migration;
- partition transfer state machine и zero-error graceful draining;
- hot-partition detection и load-aware placement;
- автоматическое восстановление после необъявленного crash активного владельца;
- Kafka, Redis, predictive scaling и ML;
- production multi-node sizing и remote deployment CI.

Прямой HPA не должен изменять StatefulSet независимо от reconciler: при scale-in он может удалить активного владельца до публикации нового assignment. CPU-only policy через тот же reconciler допустима как будущий baseline, но не называется HPA. Native HPA требует отдельного безопасного desired-state integration после этого эпика.

## Обязательные тестовые сценарии

- принятие decision только при совпадении assignment epoch и replica count;
- повтор одного decision и единственность активной операции;
- allocator golden tests и полнота каждого snapshot;
- `3 → 4`: readiness до публикации assignment, один новый epoch;
- timeout новой ноды: прежний assignment сохранён;
- CAS conflict во время scale-out;
- `4 → 3`: victim исключён из assignment до Kubernetes resize;
- Router convergence timeout: victim Pod остаётся запущенным;
- stale Router request после публикации получает `409`, stale write не выполняется;
- рестарт Control Plane в каждой изменяющей состояние фазе;
- Kubernetes timeout, conflict, forbidden и unavailable;
- новый actionable decision во время незавершённой операции;
- отказ Prometheus во время operation не подменяет topology state;
- регрессия Assignment API, Router refresh, fencing, CRUD и Compose;
- проверка RBAC: Control Plane может менять только target scale и читать необходимые Pods, остальные приложения не получают token.

## Критерии приёмки

1. Реальный k3s переход `3 → 4 → 3` выполняется по multi-metric decision без ручного изменения StatefulSet или assignment.
2. На каждом устойчивом состоянии StatefulSet desired/ready replicas, active assignment nodes и Router topology совпадают.
3. Каждый переход повышает assignment epoch ровно один раз и не создаёт неполный snapshot.
4. Scale-in никогда не удаляет ноду, ещё присутствующую в Router assignment.
5. Рестарт Control Plane в середине перехода приводит к продолжению или безопасной остановке, а не ко второй операции.
6. CRUD и fencing invariants сохраняются; все наблюдавшиеся transient `409` и `5xx` отражены в validation evidence.
7. Grafana показывает decision, phase, Kubernetes desired/ready, authoritative nodes и convergence timeline без подмены missing data нулями.
8. `make check`, контейнерные integration tests, `make test-all`, Helm lint/render и server-side dry-run проходят.
9. Compose остаётся advisory и не требует Kubernetes API.
10. Команды, фактические параметры, failure scenarios и результаты записаны в `validation.md`; незапущенные benchmark не называются результатами.

## Порядок реализации

1. Добавить domain state, PostgreSQL repository и fake ports для reconciliation.
2. Реализовать allocator и cross-language golden tests.
3. Реализовать pure transition planner и тесты restart/idempotency без Kubernetes.
4. Добавить Router topology status и HTTP observer.
5. Реализовать Kubernetes adapter и минимальный RBAC.
6. Подключить scale-out, затем safe scale-in.
7. Добавить API, метрики, config и Grafana timeline.
8. Выполнить unit/integration/Compose regression.
9. После подтверждения доступности WSL2 выполнить k3s failure matrix и `3 → 4 → 3` acceptance.
10. Зафиксировать только выполненные проверки в `validation.md` и обновить текущую архитектуру после реализации.
