# Helm chart layout

`deploy/kubernetes/` — единый application chart для namespace `autoscale-storage`. Runtime-компоненты организованы вертикально: каталог компонента содержит workload, Service и component-specific RBAC, по одному Kubernetes resource на source manifest.

| Каталог | Ответственность |
|---|---|
| `foundation/` | Namespace для локального bootstrap и неприменяемый напрямую пример Secret |
| `access/` | Scoped operator access и общий ServiceAccount приложений |
| `postgres/` | PostgreSQL Service и StatefulSet |
| `control-plane/` | Control Plane Service и Deployment |
| `processing-unit/` | Headless Service и StatefulSet |
| `router/` | Router Deployment, внутренний Service и TLS Ingress |
| `assignment-bootstrap/` | Идемпотентный Job публикации исходного assignment |
| `postgres-exporter/` | Exporter Service и Deployment |
| `kube-state-metrics/` | Workload, Service и namespace-scoped RBAC |
| `prometheus/` | StatefulSet, Service и необходимый RBAC |
| `grafana/` | Deployment и Service |
| `load/` | Suspend CronJob и workload config |
| `templates/` | Helm composition и отдельные ConfigMap templates |

`templates/resources.yaml` читает component source manifests как chart files и обрабатывает их через `tpl`. Это сохраняет один resource на файл и позволяет использовать `.Values` внутри component directories без второй копии manifests.

## Values и конфигурационные файлы

`values.yaml` задаёт images, external Secret names, Ingress и storage. `values.schema.json` запрещает неизвестные поля и проверяет типы. `values-k3s.yaml` содержит локальные overrides.

Application configs, SQL migrations, exporter queries и Grafana artifacts остаются каноническими файлами своих модулей. `k3s.ps1` передаёт их chart через `--set-file`; отдельные ConfigMap templates помещают содержимое в release. Checksum-аннотации на Pod templates инициируют rollout при изменении файлов.

Пароли и private key не передаются через values или `--set`. Скрипт до Helm создаёт:

- `storage-credentials` из `POSTGRES_PASSWORD` и `MONITOR_PASSWORD` локального `.env`;
- `router-tls` из self-signed certificate под игнорируемым `results/kubernetes/tls/`.

Chart получает только `existingSecrets.credentials` и `existingSecrets.routerTls`. Production должен подготовить Secrets с тем же контрактом через secret manager и доверенный certificate issuer.

## Внешняя граница

Единственный публичный application endpoint — `https://api.autoscale-storage.localhost`. Router Ingress публикует только `/create`, `/get`, `/update` и `/delete`; operational endpoints и остальные Services остаются внутри кластера. В production hostname и TLS Secret задаются values окружения. Ingress не заменяет ещё не реализованные authentication, authorization и tenant isolation.

## Локальный запуск

Требуются WSL2, Helm внутри WSL, рабочий alias `k3s-kubectl`, Docker Desktop и заполненный `.env`.

```powershell
make k3s-up
make k3s-status
```

`k3s-up` использует `helm upgrade --install --atomic --wait --wait-for-jobs`. Флаг `--take-ownership` предназначен для первой миграции уже существующих ресурсов из прежнего Kustomize deployment в Helm release.

При первой миграции скрипт также удаляет оставшийся в live Prometheus StatefulSet volume прежних recording rules и после успешного rollout очищает только известные hashed ConfigMap от Kustomize. Канонические файлы и новые ConfigMap Helm при этом сохраняются.

Windows не выполняет wildcard resolution для `*.localhost`. Адрес Ingress можно получить командой:

```bash
k3s-kubectl --context autoscale-storage get ingress router -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
```

Локальное приложение должно сопоставить полученный адрес с `api.autoscale-storage.localhost` через hosts/DNS. Для разовой проверки подходит `curl --resolve`; после перезапуска WSL адрес следует проверить повторно. Клиенту также нужно доверять `results/kubernetes/tls/router.crt`.

`assignment-bootstrap` является обычным Job при initial install, чтобы публикация assignment могла разблокировать readiness Router до завершения Helm install. При upgrade тот же manifest становится идемпотентным `post-upgrade` hook; Helm удаляет предыдущий hook Job перед следующим запуском, а Job самостоятельно не меняет topology после уже опубликованного epoch.
