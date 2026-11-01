# Эпик 6 — runtime validation

Дата проверки: 24 октября 2026 года. Среда: локальный WSL2 k3s, namespace `autoscale-storage`, Helm release `autoscale-storage` revision 8.

## Проверенный инкремент

- Control Plane работает в `kubernetes` actuation mode, а Compose default остаётся `advisory`.
- Control Plane ServiceAccount может читать target StatefulSet и Pods и обновлять только `statefulsets/scale`; чтение `postgres` StatefulSet и Secrets, а также удаление Pods запрещены.
- Migration Job создал persistent lifecycle tables и завершился до assignment bootstrap.
- Prometheus обнаруживает Processing Unit через namespace-scoped Pod discovery, поэтому новые ordinal входят в telemetry без статического списка.
- Helm upgrade сохраняет фактический desired replica count существующего Processing Unit StatefulSet и больше не конкурирует с actuator.
- Reconciliation API возвращает active и latest operation и нейтральный fleet snapshot. Метрики публикуют operation direction/phase/status/duration, source/target, fleet desired/observed/ready и assignment/fleet drift без Pod и operation ID labels.

## Фактический цикл

Основной acceptance выполнен только через multi-metric policy и reconciler, без ручного изменения assignment:

1. Исходное состояние: assignment epoch 2, StatefulSet `3/3`, Router nodes `processing-unit-1..3`.
2. Controller workload подтвердил scale-out. Operation `out 3 → 4` выполнялась с `17:04:50Z` до `17:05:03Z`; новый Pod стал Ready до публикации assignment, Router установил epoch 3.
3. Low workload подтвердил scale-in. Operation `in 4 → 3` выполнялась с `17:20:33Z` до `17:20:43Z`; assignment epoch 4 исключил `processing-unit-4` до уменьшения StatefulSet.
4. Финальное устойчивое состояние после повторных Helm upgrades: StatefulSet desired/ready `3/3`, fleet desired/observed/ready `3/3/3`, assignment/fleet drift `0`, Router epoch 6 и nodes `processing-unit-1..3`.

Persisted история дополнительно содержит успешные `in 3 → 2` и `out 2 → 3`. Первый переход обнаружил, что старый chart повторно выставлял статические три replicas на upgrade; chart исправлен через сохранение live StatefulSet replica count. Обратный переход подтвердил restart-safe продолжение через тот же reconciler. Все четыре операции имеют status `succeeded` и phase `completed`.

Финальный двухминутный low-load run для `4 → 3` завершил 5718 операций: 5673 success, 45 fenced conflicts, 0 server errors и 0 drops; p95 `9.07ms`, p99 `17.02ms`. Conflicts пришлись на смену assignment и соответствуют документированной fencing-семантике, а не потере данных.

## Найденные и исправленные runtime-дефекты

- Production layer graph повторно загружал default `config.yaml` внутри Kubernetes adapter. Adapter переведён на layer, получающий уже загруженный `ActuationConfig` из composition root.
- Статический Prometheus target list `processing-unit-1..3` делал telemetry неполной после scale-out. Он заменён Pod discovery с label/phase filtering.
- Неожиданный defect одного controller cycle мог завершить polling fiber. Цикл теперь публикует degraded/`controller_cycle_failed`, сбрасывает confirmation и продолжает polling; это покрыто unit test.
- Helm upgrade возвращал `.spec.replicas` к трём независимо от runtime desired state. StatefulSet template теперь использует live replica count существующего workload и initial value только при отсутствии StatefulSet.
- Короткие диагностические low/high runs не всегда давали три полных окна: после окончания запросов histogram query корректно возвращал `NaN`, а policy переходила в `InsufficientData`. Acceptance использовал двухминутную измеряемую фазу; missing data не подменялись нулями.

## Проверки

- `sbt -batch scalafmtAll scalafmtCheckAll test`: 44 passed, 0 failed.
- `go -C router test ./...`: passed.
- dashboard JSON parse и `git diff --check`: passed.
- Helm lint со всеми `--set-file` inputs: passed.
- Atomic install/upgrade path, migration Job, bootstrap Job и rollouts: passed; final release revision 8.
- Runtime RBAC positive/negative matrix: passed.
- Internal Router topology and reconciliation endpoints: passed.
- Dynamic Prometheus discovery видел четыре targets при scale-out и три после scale-in.

## Dashboard

Provisioned dashboard `Control Plane / System Overview` содержит секцию `05 / Automated reconciliation`: direction, phase, result и общий график operation source/target, Kubernetes desired/ready, fleet desired/ready, authoritative assignment и drift. Для локального просмотра:

```powershell
wsl.exe -- bash -ic "k3s-kubectl --context autoscale-storage --namespace autoscale-storage port-forward service/grafana 13000:3000"
```

Открыть `http://127.0.0.1:13000/d/control-plane-overview/control-plane-system-overview?orgId=1&from=now-45m&to=now`.

## Границы результата

Это функциональная acceptance динамического membership, а не B6 comparative benchmark. Physical migration, graceful draining, crash recovery владельца и решение о Deployment остаются в следующих этапах roadmap.
