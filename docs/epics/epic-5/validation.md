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

## Финальная приёмка — 24.10.2026

- `make test-all` — PASS. Прошли форматирование, сборка, vet и unit tests четырёх Go-модулей, 33 Scala tests, race/PostgreSQL integration tests Processing Unit, race tests Router, container tests Control Plane и внешний Compose CRUD smoke. Compose stack оставлен запущенным согласно контракту target.
- Restart matrix — PASS: Router, `processing-unit-2`, Control Plane и Prometheus были пересозданы и вернулись Ready; после пересоздания `postgres-0` с тем же PVC контрольный документ сохранил точную revision и был удалён через API.
- Prometheus после restart показывал девять targets `up`; assignment во всех сценариях сохранил epoch 2, 128 partitions и три authoritative node.
- Kubernetes load acceptance сохранён под игнорируемым `results/kubernetes/acceptance/20261024/`: `constant` — 7263 successful, 0 errors, 0 dropped, p95 3.80 ms, p99 6.24 ms; `ramp` — 4362 successful, 0 errors, 0 dropped, p95 5.41 ms, p99 7.41 ms; `burst` с пиком 300 iterations/s — 10176 successful, 0 errors, 0 dropped, p95 7.21 ms, p99 12.02 ms.
- Во время контрольного Job Control Plane получил все 30 node/signal samples: completeness 1.0, missing 0, все десять `storage_control_signal_present` равны 1, controller `healthy`, decision `Maintain`. После истечения activity windows dashboard ожидаемо возвращается в `InsufficientData`, не подменяя отсутствующие series нулями.
- Штатный workflow архивирования исправлен и проверен: `hold: true` удерживает Pod после записи результатов, scoped operator получает только дополнительный namespace subresource `pods/exec`, `kubectl cp` копирует `config.json`, `metrics.prom`, `requests.jsonl` и `summary.json`, после чего удаляется точный Job. Runtime ServiceAccount остаётся без API token.
- Visual QA двух provisioned dashboards — PASS. `Storage / Load & Bottlenecks` показывал три Processing Unit, epoch 2, offered peak 300, completed peak 237.62 req/s, нулевые drops и только HTTP 200. `Control Plane / System Overview` корректно показал timeline workload/controller/decision, а Kubernetes placement — desired/ready/available 3 рядом с authoritative nodes 3 и advisory recommendation.
- За 30-минутное окно с нагрузками и рестартами максимумы оставались ниже limits: Processing Unit около 0.07/0.5 CPU и 8.2/192 MiB каждый; Router 0.11/0.5 CPU и 8.0/192 MiB; PostgreSQL 0.20/1 CPU и 67.5/768 MiB; load generator 0.10/2 CPU и 9.0/384 MiB. Самые заметные monitoring/application значения: Control Plane 0.21/0.5 CPU и 232/384 MiB, Grafana 0.06/0.5 CPU и 190/256 MiB, Prometheus 0.07/0.5 CPU и 137/512 MiB. Текущие limits достаточны для baseline; это не sizing production-нагрузки.
- Helm rollback с ревизии 6 на содержимое ревизии 5 создал успешную ревизию 7 без потери PVC или assignment. Повторный документированный apply восстановил актуальный chart как deployed revision 8; внешний TLS Ingress CRUD после rollback/reapply — PASS.

## Обнаружено и исправлено во время первого rollout

- sbt launcher имел mode `0744`, несовместимый с runtime UID 65532; Dockerfile устанавливает `0755`.
- Именованные users внешних images не проходили `runAsNonRoot`; зафиксированы числовые UID/GID.
- PostgreSQL entrypoint требует стандартные capabilities для chown PVC и перехода к postgres UID; `drop: ALL` оставлен у остальных контейнеров, но снят у PostgreSQL.
- После исправления шаблона первый уже crashlooping PostgreSQL Pod пришлось пересоздать вручную с сохранением PVC. Последующая clean install на новом PVC подтвердила, что актуальный шаблон больше не требует этого recovery-шага.
- Helm adoption сохранял отсутствующие в chart поля прежнего Prometheus StatefulSet; точечный migration step удаляет legacy rules volume до очистки сгенерированного ConfigMap.
- Первая версия cleanup-команды неверно экранировала JSONPath на границе PowerShell/WSL. Release был успешно обновлён, но wrapper завершился ошибкой после rollout; прямой вызов `kubectl` с явным kubeconfig исправил границу, а следующая полная ревизия 4 завершилась с кодом 0.

## Clean install на пустом PVC — 24.10.2026

- Перед разрушительной проверкой создан и проверен custom-format PostgreSQL dump: 4096 документов, 186533 bytes, SHA-256 `F68A3D6A36BC0FD0B53279E44AE6C8A8AA14D214E272DC78CEA8247849E88145`. Backup хранится в ignored `results/kubernetes/acceptance/20261024/pre-clean-install/storage.dump`.
- Первый запуск на пустом namespace выявил цикл готовности: Router требовал epoch 2, а `post-install` bootstrap hook ожидал готовности обычных ресурсов Helm. `--atomic` корректно удалил неуспешный release; пустые PVC и namespace были удалены перед повтором.
- Bootstrap template исправлен: на initial install он является обычным Job и запускается параллельно с workload, на upgrade становится идемпотентным `post-upgrade` hook. Install/upgrade render-профили и `helm lint` прошли.
- Повторный документированный `make k3s-up` с полной сборкой и импортом четырёх image создал новый namespace и новые PVC `d6083043-444d-4b8c-a04f-e6d3992b7dc3` / `d5aa4196-8f3b-4899-8764-bbdcd0492baa`. Helm revision 1 завершилась `deployed` без ручного изменения ресурсов; bootstrap Job опубликовал epoch 2, 128 partitions и три node, все постоянные workload достигли Ready.
- Свежая таблица `documents` содержала 0 строк; внешний TLS CRUD create/get/delete через Ingress — PASS.
- После проверки из backup восстановлены все 4096 исходных документов; один восстановленный документ успешно прочитан через Router с сохранённой revision. Assignment bootstrap data нового стенда не перезаписывались.
- Повторный `Apply -SkipBuild` создал deployed revision 2, успешно выполнил `post-upgrade` bootstrap и сохранил данные/epoch. Таким образом initial install и последующий upgrade проверены одним актуальным chart.

Все критерии эпика 5 выполнены.

Нагрузочные числа выше являются acceptance evidence локального single-node baseline, а не сравнительным performance result и не production sizing.
