# Эпик 5: валидация k3s baseline

## Выполнено 12.10.2026

- Подтверждены WSL2 Ubuntu, активный/enabled k3s `v1.35.5+k3s1`, Ready control-plane node и рабочий alias `k3s-kubectl`.
- Kustomize render прошёл локально; весь набор ресурсов принят k3s API через server-side dry-run.
- Локальные image Processing Unit, Router, Control Plane и load generator собраны и импортированы в containerd k3s.
- Все постоянные workload достигли Ready: PostgreSQL, три Processing Unit, Control Plane, Router, Prometheus, Grafana, kube-state-metrics и PostgreSQL exporter.
- Assignment bootstrap завершён; активен epoch 2 с 128 partitions и тремя адресами `processing-unit-N.processing-unit-headless:8080`.
- Одноразовый smoke-Pod без Kubernetes API token выполнил create/get/update/delete через Router — PASS.
- В предыдущей итерации Prometheus recording rules вернули CPU/RAM series со всеми тремя application node labels, а `kube_statefulset_status_replicas_ready` и `kube_statefulset_status_replicas_available` были равны 3 — PASS для прежней реализации.
- В idle `storage_control_data_completeness_ratio = 0.6`: отсутствующие request/latency series не превращались в нули, поэтому это было ожидаемое non-actionable состояние до нагрузки.
- Scoped context читает Pod namespace и не имеет доступа `get nodes` — PASS.
- Текущий пользовательский context остался `default`; создание `autoscale-storage` не выполняет `use-context` — PASS.
- Повторный `Apply -SkipBuild` прошёл без ручного восстановления: epoch 2 не изменился, bootstrap завершился идемпотентно, rollout всех изменяемых application workload и Grafana — PASS.
- `make k3s-status` обновил короткоживущий context и показал все постоянные workload Ready, PVC Bound и bootstrap Complete — PASS.
- `promtool` подтверждал прежние Compose/k3s recording rules — исторический PASS; эти rules затем заменены Docker/Kubernetes реализациями `PrometheusQuerySet`.

## Helm и текущий runtime — 24.10.2026

- В WSL подтверждён Helm `v3.21.0`; `--take-ownership` поддерживается, доступ к k3s через `/home/artrsyf/.kube/k3s.yaml` работает.
- `helm lint` завершён без ошибок. `helm template` сформировал 40 ресурсов, которые приняты текущим Kubernetes API через server-side dry-run.
- Первый `helm upgrade --install --atomic --wait --wait-for-jobs --take-ownership` принял существующие ресурсы прежнего Kustomize deployment. Release `autoscale-storage` находится в состоянии `deployed`; финальная проверенная ревизия — 4.
- Повторные `Apply -SkipBuild` завершились без ручного изменения workload. Bootstrap hook выполнился повторно, но идемпотентно сохранил epoch 2, 128 partitions и три Processing Unit.
- После adoption обнаружен оставшийся в live Prometheus StatefulSet volume старого `prometheus-rules-*`. Скрипт миграции теперь удаляет этот volume, ждёт новый Ready Pod и только затем удаляет точно распознанные hashed ConfigMap Kustomize. Активные ConfigMap имеют стабильные Helm-имена; PVC и данные PostgreSQL не удалялись.
- Все постоянные workload Ready. После финального upgrade новые Pod Control Plane, Router, Prometheus и Processing Unit имели нулевой restart count; девять Prometheus scrape targets находились в состоянии `up`.
- `KubernetesPrometheusQuerySet` проверен через опубликованные Control Plane metrics: CPU и memory присутствуют, нормализованы для активных Processing Unit и участвуют в aggregate. В idle completeness остаётся `0.6`, decision — `InsufficientData`, потому что histogram series DB/request latency без наблюдений отсутствуют, а не подменяются нулём.
- Traefik Ingress получил адрес `172.19.39.154` и self-signed TLS certificate для `api.autoscale-storage.localhost`. Внешний Windows `curl` с доверием к leaf certificate и явным `--resolve` выполнил create/get/update/delete — PASS; `/readyz` снаружи вернул 404 — PASS.
- Windows не резолвит `api.autoscale-storage.localhost` автоматически. Это не дефект Ingress: локальному клиенту требуется hosts/DNS-сопоставление с текущим адресом Ingress; после перезапуска WSL адрес нужно проверить повторно. Production должен предоставить обычный DNS и доверенный certificate issuer.
- `make build vet test` — PASS после Helm rollout: сборка всех модулей, Go vet, тесты четырёх Go modules/packages и 33 Scala tests. Изменённые и новые Scala-файлы отдельно приведены к текущему scalafmt.

Исторические проверки Kustomize и recording rules выше описывают первоначальный rollout. Актуальный runtime использует Helm и отдельные Docker/Kubernetes реализации `PrometheusQuerySet`; старые rules и Kustomize composition удалены.

## Обнаружено и исправлено во время первого rollout

- sbt launcher имел mode `0744`, несовместимый с runtime UID 65532; Dockerfile устанавливает `0755`.
- Именованные users внешних images не проходили `runAsNonRoot`; зафиксированы числовые UID/GID.
- PostgreSQL entrypoint требует стандартные capabilities для chown PVC и перехода к postgres UID; `drop: ALL` оставлен у остальных контейнеров, но снят у PostgreSQL.
- После исправления шаблона первый уже crashlooping PostgreSQL Pod пришлось пересоздать вручную с сохранением PVC. Clean apply ещё должен подтвердить, что этот recovery-шаг больше не нужен.
- Helm adoption сохранял отсутствующие в chart поля прежнего Prometheus StatefulSet; точечный migration step удаляет legacy rules volume до очистки сгенерированного ConfigMap.
- Первая версия cleanup-команды неверно экранировала JSONPath на границе PowerShell/WSL. Release был успешно обновлён, но wrapper завершился ошибкой после rollout; прямой вызов `kubectl` с явным kubeconfig исправил границу, а следующая полная ревизия 4 завершилась с кодом 0.

## Не выполнено

- Длительные `constant`, `ramp`, `burst` нагрузки и архивирование результатов.
- Restart/failure matrix для Router, Processing Unit, Control Plane, Prometheus и PostgreSQL.
- Визуальная проверка новых Kubernetes-панелей Grafana.
- Полный Compose regression и `make test-all`. `make fmt-check` подтверждает gofmt, но общий `scalafmtCheckAll` остаётся красным на накопленном наборе неизменённых Scala-файлов; файлы текущего change set отформатированы отдельно.
- Экспериментальная калибровка requests/limits и измерение monitoring/load-generator headroom.

Текущий smoke подтверждает функциональную связность baseline, но не является performance result и не закрывает эпик целиком.
