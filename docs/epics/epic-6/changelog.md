# Эпик 6: Kubernetes actuation и reconciliation

## Статус

Ветка `epic/6-kubernetes-actuation` создана от `master` после merge завершённого эпика 5. Основной кодовый и k3s acceptance-срез реализован; фактические команды, переходы и найденные runtime-дефекты записаны в [validation report](validation.md). Канонический scope и инварианты находятся в [README](README.md).

## Реализовано

- добавлены доменные типы fleet и persisted-operation lifecycle без Kubernetes types;
- ошибки allocator находятся в `reconciliation/domain`, рядом с остальной доменной моделью;
- добавлен потребительский порт `PartitionAllocator` и детерминированная Scala-реализация fixed logical partitions;
- allocator использует только immutable collections, сохраняет максимально возможное число владельцев, балансирует partitions с разницей не более одной и повышает epoch ровно на один;
- golden unit tests подтверждают минимальные 50 переназначений для `3 → 5` и `5 → 3`, полноту, баланс и детерминизм;
- Router предоставляет cluster-internal `GET /topology` с установленными epoch, partition count и node IDs;
- topology endpoint подключён отдельно от публичных document routes и не опубликован Ingress;
- добавлена migration `004_scaling_operations.sql` с единственностью активной операции, optimistic version и нормализованным хранением planned assignment;
- добавлены `ScalingOperationRepository`, PostgreSQL adapter и `ScalingOperationService` с проверкой исходного epoch/replica count;
- lifecycle применяет разные допустимые последовательности фаз для scale-out и scale-in, отклоняет multi-step и non-actionable decisions и сохраняет классифицированные ошибки в domain;
- добавлены Kubernetes-neutral порты `ProcessingUnitFleet` и `RouterTopologyObserver`; fleet adapter обязан заранее подтвердить survivors для scale-in;
- добавлена строгая actuation-конфигурация с безопасным default `advisory`, интервалами reconciliation, Router URL и инфраструктурным placement contract;
- реализован типизированный HTTP observer Router topology со строгим snake_case DTO, timeout, protocol/data classification и запретом duplicate/unknown fields;
- реализован pure `TopologyReconciler`: scale-out выполняет capacity → readiness → assignment → Router, scale-in — assignment → Router → capacity;
- end-to-end unit tests проверяют фактический порядок side effects для переходов `3 → 4` и `4 → 3` на fake-портах;
- официальный Kubernetes Java client `26.0.1`, соответствующий Kubernetes `1.35`, изолирован в StatefulSet fleet adapter;
- Kubernetes adapter читает точные Ready Pod identities, прогнозирует survivors по ordinal и изменяет только scale subresource;
- Control Plane composition запускает отдельный actuation loop только в режиме `kubernetes`; Compose остаётся `advisory`;
- Helm выдаёт Kubernetes API token только отдельному Control Plane ServiceAccount и ограничивает Role чтением target StatefulSet/Pods и обновлением `statefulsets/scale`;
- идемпотентный migration Job применяет `004_scaling_operations.sql` как обычный install resource и `post-upgrade` hook до assignment bootstrap;
- добавлен cluster-internal `GET /control/reconciliation` с active/latest persisted operation и нейтральным fleet snapshot;
- добавлены bounded reconciliation/fleet metrics и Grafana timeline, сопоставляющий operation с desired/ready Pod и authoritative assignment;
- Prometheus Processing Unit targets переведены со статического списка на namespace-scoped Pod discovery;
- Helm upgrade сохраняет live replica count существующего StatefulSet и использует три replicas только как initial value;
- controller loop восстанавливается после неожиданного defect отдельного cycle и не оставляет polling без supervision;
- migration передаётся в Helm через обязательный `--set-file` contract и проверяется render job CI;
- полный Scala suite содержит 44 успешных теста; полный Router Go suite проходит;
- локальный k3s подтвердил основной `3 → 4 → 3`, direction-specific safety order, epoch `2 → 3 → 4`, migration/RBAC/API/metrics и нулевой итоговый drift; стенд оставлен в состоянии `3/3`, Router epoch 6 после дополнительной проверки restart/upgrade convergence.

## Точка старта

- Kubernetes baseline использует StatefulSet `processing-unit` с ordinal `1`, стабильными node IDs и headless DNS.
- Active assignment epoch 2 содержит три Processing Unit и остаётся источником topology.
- Control Plane выдаёт advisory `ScaleOut`, `ScaleIn`, `Maintain` или `InsufficientData`, но не изменяет инфраструктуру.
- Runtime allocator, Kubernetes client, membership reconciliation и Router topology acknowledgement отсутствуют.
- Все application ServiceAccount, включая Control Plane, не имеют API token; новый доступ должен быть выдан только отдельному Control Plane ServiceAccount.
- Compose и k3s acceptance эпика 5 завершены; их результаты не являются benchmark динамического масштабирования.

## Планируемая поставка

1. Persisted desired state и restart-safe `ScalingOperation`.
2. Нейтральные порты `ProcessingUnitFleet`, `PartitionAllocator` и `RouterTopologyObserver`.
3. Scala allocator fixed logical partitions с golden tests эпика 2.
4. Namespace-scoped Kubernetes adapter и минимальный RBAC для StatefulSet scale и Pod readiness.
5. Последовательный scale-out: replicas → readiness → assignment → Router convergence.
6. Последовательный scale-in: assignment → Router convergence → replicas.
7. Read-only reconciliation API, bounded metrics и Grafana timeline.
8. Advisory default для Compose и явный Kubernetes mode в Helm.
9. Unit, PostgreSQL integration, Compose regression и k3s `3 → 4 → 3` acceptance.

## Зафиксированные границы

- StatefulSet сохраняется на протяжении эпика 6.
- Kubernetes types остаются внутри infrastructure adapter.
- Прямой HPA, B6 benchmark, physical migration и graceful draining не входят в change set.
- Переход на Deployment рассматривается в D6 `Failure Recovery` и остаётся условным архитектурным решением.
- Функциональный scope эпика зафиксирован; детальный code review продолжается без расширения реализации.
