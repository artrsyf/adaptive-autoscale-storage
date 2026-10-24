# Эпик 5: изменения ветки и продолжение работы

## Реализовано

- Целевая среда изменена с отдельного kind на существующий single-node k3s в WSL2.
- Добавлены namespace `autoscale-storage`, исходные Kubernetes manifests, scoped operator context и минимальные runtime-доступы; первоначальная Kustomize composition позднее заменена Helm chart.
- Три Processing Unit развёрнуты StatefulSet с ordinal 1–3 и headless DNS; Pod name, application node ID и assignment node ID совпадают.
- Добавлены PostgreSQL/Prometheus PVC, application probes, resource requests/limits, kube-state-metrics, PostgreSQL exporter, Grafana и suspend load CronJob.
- Assignment bootstrap публикует epoch 2 через HTTP API; replica count Kubernetes не меняет authoritative assignment.
- Введён общий инфраструктурный контракт `PrometheusQuerySet`, возвращающий полный набор запросов по доменным `SignalName`.
- `DockerPrometheusQuerySet` сохраняет исходные Compose/cAdvisor PromQL и выбирается по умолчанию, поэтому существующий CLI и Compose-конфигурация обратно совместимы.
- `KubernetesPrometheusQuerySet` инкапсулирует namespace, Pod/container labels и kube-state-metrics; k3s manifest выбирает его аргументом `--metrics-profile kubernetes`.
- Промежуточные recording rules `storage_node_cpu_utilization` и `storage_node_memory_utilization` удалены: нормализация в `node` выполняется реализациями query contract, а domain/service слои не ветвятся по среде.
- Kubernetes manifests реорганизованы из крупных `applications.yaml`, `monitoring.yaml`, `access.yaml` и `postgres.yaml` в вертикальные component directories. Router, Control Plane, Processing Unit, PostgreSQL, exporters, Prometheus, Grafana и workload теперь владеют своими ресурсами; в каждом source manifest находится один объект, а ConfigMap создаются общими Helm templates.
- Router получил единственный внешний application entrypoint через Traefik TLS Ingress `api.autoscale-storage.localhost`. Наружу маршрутизируются только четыре CRUD path; локальный `router-tls` генерируется в ignored results, а production сохраняет Secret contract и заменяет сертификат/hostname через deployment environment.
- Kubernetes delivery переведён с Kustomize на единый Helm application chart. Component resource-файлы сохранены раздельными; images, Ingress, external Secret names и storage вынесены в typed values, а канонические configs/migrations/dashboards передаются через `--set-file` с checksum rollout.
- Assignment bootstrap работает обычным Job на initial install, чтобы не блокироваться на readiness Router, и идемпотентным `post-upgrade` hook на обновлениях; предыдущий hook Job удаляется перед новым запуском.
- Первый Helm release принял существующие ресурсы через `--take-ownership`; migration cleanup удаляет legacy Prometheus rules volume и только известные hashed ConfigMap прежнего Kustomize deployment.
- Проверены повторный upgrade, TLS Ingress, внешний CRUD, изоляция `/readyz`, девять Prometheus targets и Kubernetes CPU/RAM query profile; Windows-клиенту требуется явное локальное DNS/hosts-сопоставление hostname с адресом Ingress.
- Исправлен executable mode launcher Control Plane в runtime image.
- Добавлены `make k3s-up` и `make k3s-status`.
- Полная приёмка закрыла Compose regression, Kubernetes restart matrix, три нагрузочных сценария, visual QA Grafana, resource headroom и Helm rollback/reapply; доказательства записаны в `validation.md`.
- Load Job теперь остаётся Running после формирования результатов, а namespace-scoped operator имеет необходимый `pods/exec` для `kubectl cp`; runtime ServiceAccount приложений по-прежнему tokenless.
- Разрушительная clean-install проверка выявила и устранила deadlock `post-install` bootstrap с readiness Router. Актуальный chart использует обычный bootstrap Job на install и `post-upgrade` hook на обновлении; повторный install на новых PVC и revision 2 прошли автоматически, исходные документы восстановлены из проверенного backup.

## Продолжение

1. Добавить базовый GitHub CI для форматирования, сборки, vet и unit tests без автоматического deployment.
2. Не переносить локальные baseline limits в production без отдельного sizing на целевой multi-node среде.

Change set зафиксирован в ветке `epic/5-kubernetes-baseline`; merge в `master` не выполнялся.
