# Эпик 5 — k3s static baseline и placement contract

## Цель и фактическая среда

Эпик переносит существующий стенд в уже установленный локальный single-node k3s под WSL2 и создаёт статическую baseline-конфигурацию для будущего сравнения способов масштабирования. Отдельный kind-кластер не создаётся: он дублировал бы работающий container runtime и не добавлял бы исследовательской ценности.

Эпик продолжает завершённый [наблюдающий Control Plane v1](../epic-4/README.md) и закрывает инфраструктурный фундамент B2. HPA, multi-metric actuator, автоматическая перестройка assignment, draining и безопасный scale-in остаются последующими инкрементами.

## Реализованный поток

```text
локальный k3s / WSL2
    ↓ namespace autoscale-storage + scoped operator context
Helm application chart + локальные application images в containerd k3s
    ↓
PostgreSQL StatefulSet + Control Plane
    ↓ HTTP bootstrap assignment epoch 2
processing-unit-1..3 StatefulSet + headless DNS
    ↓
external client → TLS Ingress → Router → Processing Units → PostgreSQL fence
    ↓
kubelet/cAdvisor + kube-state-metrics → KubernetesPrometheusQuerySet
Docker cAdvisor                    → DockerPrometheusQuerySet
                                      ↓
                       MetricsSource / SignalName → advisory policy → Grafana
```

## Граница Kubernetes

Kubernetes API не является зависимостью бизнес-кода. Router, Processing Unit, Control Plane, PostgreSQL exporter, Grafana и bootstrap Job используют ServiceAccount без API-токена. Они видят только DNS, HTTP-контракты, PostgreSQL и Prometheus.

Внешнее приложение входит через `https://api.autoscale-storage.localhost` → Traefik Ingress → внутренний Router Service. Ingress завершает TLS и публикует только `/create`, `/get`, `/update`, `/delete`; operational endpoints и все остальные компоненты остаются cluster-internal. Локальный `router-tls` создаётся скриптом как self-signed сертификат, а production-среда должна предоставить Secret с тем же контрактом от доверенного issuer и собственный hostname overlay. Это формирует SaaS-compatible network boundary, но не объявляет ещё не реализованные authentication, authorization и tenant isolation.

Prometheus — единственный runtime-компонент с cluster-scoped доступом: ему разрешено только читать node metadata и kubelet proxy endpoints для cAdvisor. Доступ kube-state-metrics ограничен Role в `autoscale-storage`. Локальный context `autoscale-storage` получает короткоживущий токен namespace-scoped ServiceAccount и не читает cluster nodes.

Control Plane использует общий `PrometheusQuerySet`, который обязан предоставить запрос для каждого доменного `SignalName`. Compose сохраняет прежние cAdvisor PromQL и остаётся профилем по умолчанию; Kubernetes deployment явно выбирает `--metrics-profile kubernetes`. Обе реализации преобразуют environment-specific labels в общий `node`, поэтому `MetricsSource`, агрегатор и policy не ветвятся по среде. Kubernetes DTO, Pod, StatefulSet и Deployment не представлены в domain/service слоях.

## Placement и identity contract

| Понятие | Значение |
|---|---|
| Kubernetes workload | StatefulSet `processing-unit`, три статические replica |
| Pod identity | `processing-unit-1`, `processing-unit-2`, `processing-unit-3`; `.spec.ordinals.start: 1` |
| Application node ID | точное имя Pod из Downward API, переданное через `--node` |
| Service discovery | `<pod>.processing-unit-headless:8080` |
| Assignment node ID | совпадает с application node ID |
| Источник истины | опубликованный assignment snapshot в PostgreSQL, не replica count Kubernetes |

Миграция создаёт исходный Compose-совместимый epoch 1. Идемпотентный bootstrap Job вызывает Assignment HTTP API и публикует epoch 2 с адресами StatefulSet. При initial install это обычный Helm resource, который запускается параллельно с workload и разрывает зависимость готовности Router от assignment; при upgrade тот же шаблон становится `post-upgrade` hook. При epoch больше 1 Job не меняет topology. Это промежуточное ограничение существующего API: пустое assignment-состояние пока не поддерживается.

## Состав поставки

- [`deploy/kubernetes/Chart.yaml`](../../../deploy/kubernetes/Chart.yaml) определяет единый application chart; вертикальные каталоги по runtime-компонентам содержат workload/Service/RBAC по одному resource на файл, а общие templates создают ConfigMap. Карта chart находится в [`deploy/kubernetes/README.md`](../../../deploy/kubernetes/README.md);
- `deploy/kubernetes/k3s.ps1` проверяет `.env`, собирает четыре локальных image, импортирует их в containerd k3s, применяет credential Secret, создаёт/переиспользует локальный TLS Secret для Router Ingress, выполняет atomic `helm upgrade --install`, удаляет точно распознанные артефакты прежнего Kustomize deployment, обновляет scoped context и ждёт rollout; private key хранится только под игнорируемым `results/`;
- application configs, migrations, exporter queries и dashboards поступают из канонических файлов через `--set-file`; Helm values хранят только имена существующих Secrets, images, Ingress и storage settings;
- PostgreSQL и Prometheus используют PVC `local-path`; Processing Units используют StatefulSet; остальные runtime-компоненты — Deployment;
- startup/readiness/liveness probes используют фактические operational endpoints;
- CronJob `load` хранится с `suspend: true` и служит шаблоном для отдельных load Jobs; после прогона генератор остаётся запущенным, чтобы результаты можно было скопировать до явного удаления Job;
- Grafana показывает Kubernetes desired/ready/available replicas рядом с authoritative node count и advisory recommendation;
- Docker Compose остаётся быстрым regression-стендом; запуск без `--metrics-profile` использует прежние Docker cAdvisor-запросы, поэтому существующий способ запуска совместим.

## Команды

WSL2 и локальный k3s должны быть запущены. Kubernetes-команды выполняются WSL-алиасом `k3s-kubectl`; Helm должен быть установлен внутри WSL и использует kubeconfig из этого alias без переключения текущего context.

```powershell
make k3s-up
make k3s-status
```

Скрипт не меняет текущий context `default`; он создаёт/обновляет отдельный context `autoscale-storage`. Для прямой проверки:

```bash
k3s-kubectl --context autoscale-storage get pods
k3s-kubectl --context autoscale-storage port-forward service/grafana 3000:3000
```

Одноразовый нагрузочный Job создаётся из suspend CronJob с уникальным именем. После записи итогового `summary.json` генератор продолжает отдавать метрики и оставляет Pod в состоянии Running. Результаты нужно скопировать из контейнера `load` до явного удаления Job; они не считаются принятыми автоматически.

```bash
k3s-kubectl --context autoscale-storage create job --from=cronjob/load load-constant
load_pod=$(k3s-kubectl --context autoscale-storage get pod -l job-name=load-constant -o jsonpath='{.items[0].metadata.name}')
k3s-kubectl --context autoscale-storage cp "${load_pod}:/results" ./results/kubernetes/load-constant -c load
k3s-kubectl --context autoscale-storage delete job load-constant
```

Scoped operator получает `pods/exec` только для обслуживающих операций внутри namespace; это требуется механизму `kubectl cp`. Runtime ServiceAccount приложений по-прежнему не имеет API token и не получает этот доступ.

## Статус и критерии приёмки

На 24.10.2026 эпик принят полностью. Chart проходит initial install на пустом namespace/PVC и повторный upgrade без ручной коррекции Pod; rollback, TLS Ingress, внешний CRUD, `constant`/`ramp`/`burst`, полная restart matrix, visual QA и Compose regression пройдены. Kubernetes Prometheus profile возвращает CPU/RAM для трёх Processing Unit, активный assignment сохраняет epoch 2 и 128 partitions. Фактические доказательства и ограничения перечислены в [validation.md](validation.md), состав изменений — в [changelog.md](changelog.md).

Выполненные критерии приёмки:

1. повторного clean apply документированной командой без ручной коррекции Pod;
2. `constant`, `ramp` и `burst` прогонов с сохранёнными артефактами;
3. проверки рестартов Router, одной Processing Unit, Control Plane, Prometheus и PostgreSQL;
4. визуальной проверки новых Grafana-панелей;
5. Compose regression (`make test-all`) после Kubernetes-изменений;
6. экспериментальной калибровки requests/limits и проверки headroom генератора и мониторинга.

## Вне scope

- HPA и любое автоматическое изменение replica count;
- Kubernetes API client или actuator внутри Control Plane;
- автоматическое обнаружение membership и публикация assignment;
- allocator, rebalancing, draining и физическая миграция данных;
- безопасный динамический scale-in;
- Kubernetes Operator, Kafka, Redis и полный сравнительный benchmark B6.

`ScalingDecision.recommendedReplicas` остаётся наблюдаемым reference value и не является desired state Kubernetes.
