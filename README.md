# Autoscale Distributed Storage

Локальный исследовательский стенд: **Scala Control Plane + Go HTTP Router → Go processing unit → PostgreSQL**, с документным API, управляемым assignment, PostgreSQL-backed fencing и наблюдаемостью. Стенд запускается через Docker Compose или как статический baseline в локальном k3s. Текущая архитектура: [docs/architecture.md](docs/architecture.md).

Полная навигация: [документация проекта](docs/README.md).

## Структура кода

```text
processing-unit/ # собственный Go-модуль, cmd, internal, config, migrations, Dockerfile
router/          # собственный Go-модуль, cmd, internal, config, Dockerfile
control-plane/   # Scala 3/ZIO-сервис публикации assignment, sbt, Dockerfile
benchmarks/      # независимые нагрузочный клиент и partitioning harness
deploy/          # инфраструктура мониторинга
docs/            # архитектура, roadmap, ВКР и документы эпиков
```

Control Plane публикует assignment, Router кэширует snapshot и выбирает исполнителя, processing unit выполняет CRUD через атомарный PostgreSQL fence. Сервисы содержат собственные модели и транспорт; общего runtime-пакета между Go и Scala нет, совместимость задаётся сетевыми контрактами.

Приложение не запускает тесты или нагрузку. Направление зависимостей и пример update описаны в [архитектуре кода](docs/architecture.md).

## Запуск

Нужны Docker Desktop с WSL2/Linux containers и Docker Compose. Для локальной разработки — Go 1.25.7, JDK 21 и sbt; Docker-сборки содержат собственные toolchain. Подготовка конфигурации генератором не требуется. Команды выполняются из корня проекта в PowerShell 7.

```powershell
Copy-Item .env.example .env
docker compose up -d --build --wait --remove-orphans
```

`.env` содержит только `POSTGRES_PASSWORD` и `MONITOR_PASSWORD`. Не перезаписывайте существующий файл с паролями. Настройки приложений находятся в `control-plane/config.yaml`, `router/config.yaml` и `processing-unit/config.yaml`, инфраструктуры — в Compose и `deploy/`.

Compose собирает отдельные образы Control Plane, Router и processing unit. Router не получает пароль PostgreSQL. После изменения подключённых YAML-файлов используйте `docker compose up -d --build --force-recreate --wait`: Compose сам не отслеживает содержимое bind mounts.

- API: http://localhost:8080
- Assignment API: http://localhost:8081/assignments/current
- Grafana: http://localhost:3000/d/storage-baseline
- Prometheus: http://localhost:9090
- Readiness: http://localhost:8080/readyz

Grafana открывается с правами просмотра; datasource и дашборд загружаются автоматически. Пустые панели до первого прогона ожидаемы. Отсутствующие измерения не подменяются нулями.

Внешние порты привязаны к loopback. Пароли-примеры в `.env.example` — публичные значения только для локального стенда; не используйте их вне него. cAdvisor имеет привилегии и read-only mounts для чтения ресурсов Docker VM. Метрики контейнеров относятся к Linux VM, а не ко всей Windows-машине.

```powershell
docker compose logs --tail 100 control-plane router processing-unit-1 postgres
docker compose down
```

Остановка сохраняет данные PostgreSQL и Prometheus. `down -v` удаляет их. Изменение пароля в `.env` не меняет пароль существующей базы: init-скрипты выполняются только на новом volume. Ротация паролей существующей БД выполняется отдельно.

SQL из `processing-unit/migrations/` применяется при инициализации нового PostgreSQL volume; это пока начальная схема, не универсальный migration runner.

### Статический k3s baseline

Для Kubernetes используется существующий k3s в WSL2, а не отдельный kind-кластер. Требуются рабочий WSL-алиас `k3s-kubectl`, Helm в WSL, Docker Desktop и заполненный `.env`.

```powershell
make k3s-up
make k3s-status
```

Развёртывание выполняется application chart из `deploy/kubernetes/` через `helm upgrade --install` и ограничено namespace `autoscale-storage`. Скрипт создаёт отдельный одноимённый context с короткоживущим namespace-scoped токеном, но не переключает текущий context. Прикладные Pod не имеют Kubernetes API token; cluster-scoped read доступ к kubelet metrics есть только у Prometheus. Внешний application endpoint — `https://api.autoscale-storage.localhost`; TLS завершается на Traefik Ingress, который публикует только четыре документных endpoint Router. Локальный CA не используется: клиенту нужно явно доверять self-signed `results/kubernetes/tls/router.crt`. Windows не резолвит этот поддомен автоматически: для внешнего локального клиента hostname нужно сопоставить с текущим адресом Router Ingress через hosts/DNS либо передать эквивалентное адресное сопоставление клиенту.

Пароли и TLS private key не передаются в Helm values: `k3s.ps1` создаёт `storage-credentials` и `router-tls` до установки release, а chart ссылается на них через `existingSecrets`. Исходные application configs, SQL migrations, exporter queries и dashboards передаются chart командой `--set-file`; их содержимое не дублируется в каталоге deployment.

```bash
k3s-kubectl --context autoscale-storage get pods
k3s-kubectl --context autoscale-storage port-forward service/grafana 3000:3000
```

Подробности identity, assignment bootstrap, доступа и текущей валидации: [Epic 5](docs/epics/epic-5/README.md).

Для volume, созданного до эпика 3, примените новую идемпотентную схему assignment до перезапуска приложений:

```powershell
docker compose exec -T postgres psql -U storage -d storage -v ON_ERROR_STOP=1 -f /docker-entrypoint-initdb.d/003_assignment.sql
```

## API

Все четыре команды — POST с JSON. Составной ключ: `(partition_key, id)`. `payload` — JSON-объект. Лимит тела — 1 MiB, каждого ключа — 256 UTF-8 байт. Неизвестные поля команды отклоняются.

```powershell
$base = 'http://localhost:8080'
$created = Invoke-RestMethod "$base/create" -Method Post -ContentType application/json -Body '{"partition_key":"tenant-a","id":"doc-1","payload":{"name":"Anna"}}'
$body = @{ partition_key='tenant-a'; id='doc-1'; expected_revision=$created.revision; payload=@{ name='Maria' } } | ConvertTo-Json
$updated = Invoke-RestMethod "$base/update" -Method Post -ContentType application/json -Body $body
Invoke-RestMethod "$base/get" -Method Post -ContentType application/json -Body '{"partition_key":"tenant-a","id":"doc-1"}'
$body = @{ partition_key='tenant-a'; id='doc-1'; expected_revision=$updated.revision } | ConvertTo-Json
Invoke-RestMethod "$base/delete" -Method Post -ContentType application/json -Body $body
```

Create возвращает 201; остальные успешные команды — 200. Delete возвращает метаданные удалённого документа без payload. Ошибка имеет форму `{"error":"revision_conflict"}`.

| Код | Значение |
|---|---|
| 400 / 413 | Некорректная команда / превышен размер |
| 404 | Нет документа или операции |
| 409 | Ключ занят, версия устарела или неверное назначение processing unit |
| 429 | Исчерпан лимит активных запросов |
| 502 / 503 | processing unit / PostgreSQL недоступны |
| 504 | Истёк deadline; результат записи может быть неопределённым |

Update полностью заменяет payload. Update/delete проверяют revision под блокировкой строки в транзакции. Новая UUID revision создаётся при каждом create/update; повторное создание не переиспользует старую версию. Ответ отправляется после commit. Автоматических повторов записи и восстановления потерянного ответа нет.

## Маршрутизация и ограничения

Partition = первые 8 байт SHA-256 от UTF-8 `partition_key`, unsigned big-endian, modulo `partition_count` активного snapshot. Control Plane хранит явную карту `partition → processing unit`, Router периодически получает её и разрешает маршруты локально. Processing unit не вычисляет владельца по ключу: PostgreSQL атомарно сверяет переданные partition, owner и epoch с активным assignment в той же транзакции, что и CRUD.

Все processing unit работают с общей таблицей PostgreSQL. Физического шардирования и миграции строк, Kafka, Redis и autoscaling пока нет. Текущий Control Plane содержит только модуль assignment, без control loop и scaling policy.

Начальные лимиты: PostgreSQL 1 CPU / 768 MiB, processing unit 0.5 CPU / 192 MiB каждый. Пул: 8 соединений на processing unit, суммарно 24. Параметры находятся в `processing-unit/config.yaml` и `docker-compose.yaml`. Readiness проверяет доступность таблицы; перегрузка ограничивается числом активных запросов, скрытой неограниченной очереди нет.

## Проверки

Корневой `Makefile` объединяет четыре независимых Go-модуля и Scala-модуль Control Plane. Нужны GNU Make, Go, JDK 21, sbt и PowerShell 7 (`pwsh`) в PATH; для контейнерных команд — работающий Docker Desktop/WSL2 и настроенный `.env`. На Windows host race-тесты дополнительно требуют CGO и C-компилятор.

GitHub Actions выполняет базовые проверки Go, Scala и Helm для каждого push и pull request без deployment и без доступа к локальному k3s. Состав jobs и намеренно исключённые stateful-проверки описаны в [docs/ci.md](docs/ci.md).

```powershell
make test          # Go-тесты четырёх модулей и Scala-тесты Control Plane
make check         # Форматирование, сборка, vet и тесты всех модулей
make test-all      # check + Docker race/integration tests + CRUD smoke текущих образов
make smoke-failure # Отдельно: проверка остановки и восстановления PostgreSQL
make help          # Все команды
```

`make test-all` прекращается на первой ошибке, пересобирает/поднимает стенд и оставляет его работающим. Volumes сохраняются. Host PostgreSQL-тесты без пароля пропускаются, но контейнерный этап передаёт пароль из `.env` и выполняет их с настоящей БД. Нагрузочные и визуальные проверки запускаются отдельно; `test-all` не является полной исследовательской серией.

Другие команды: `make build`, `make vet`, `make fmt`, `make test-race`, `make integration`, `make up`, `make down`, `make status`, `make logs`. Нагрузка: `make bench SCENARIO=constant DISTRIBUTION=hot RATE=50 SECONDS=60`. Отказ БД и нагрузку не запускайте одновременно.

Автономное сравнение алгоритмов partitioning запускается без Docker:

```powershell
make partitioning
```

Оно сравнивает modulo, consistent hashing и фиксированные logical partitions на сценариях изменения моделируемой топологии. Статический состав runtime из трёх processing unit не меняется. Каждый запуск сохраняет конфигурацию, сырые повторения, CSV, Markdown и SVG-графики в `results/partitioning/<UTC timestamp>/`; подробная методика — в [эпике 2](docs/epics/epic-2/README.md).

Эквивалентные команды без Make:

```powershell
go -C processing-unit test ./...
go -C router test ./...
Push-Location control-plane; sbt -batch test; Pop-Location
go -C processing-unit vet ./...
go -C router vet ./...
go -C benchmarks/load test ./...
go -C benchmarks/load vet ./...
go -C benchmarks/partitioning test ./...
go -C benchmarks/partitioning vet ./...
docker compose --profile test run --build --rm tests
docker compose --profile test run --build --rm router-tests
docker compose --profile test run --build --rm control-plane-tests
./benchmarks/smoke.ps1
```

Команды Compose выполняют Go-тесты с race detector и Scala-тесты Control Plane с настоящим PostgreSQL. Без `POSTGRES_PASSWORD` локальные PostgreSQL-тесты пропускаются. Проверяются CRUD-конкуренция, fencing-lock, чтение активного assignment, CAS-конфликт, маршрутизация и admission. `smoke.ps1` проверяет публичный API через Router.

## Нагрузочные эксперименты

Генератор собирается отдельно из `benchmarks/load`, читает YAML-конфигурацию и обращается к HTTP API. Каждый Go-модуль проверяется отдельно. `run.ps1` пересобирает образ генератора перед прогоном.

```powershell
./benchmarks/run.ps1 -Scenario constant -Distribution uniform -Rate 100 -Seconds 60
./benchmarks/run.ps1 -Scenario constant -Distribution hot -Rate 100 -Seconds 60
./benchmarks/run.ps1 -Scenario ramp -Rate 500 -Seconds 90
./benchmarks/run.ps1 -Scenario burst -Rate 100 -Seconds 90
./benchmarks/run.ps1 -Scenario db-lock -Rate 100 -Seconds 60 -ReadPercent 20
# Изменение состава processing unit требует нового assignment snapshot и согласованной инфраструктуры.
```

Запускайте эксперименты последовательно. Скрипт меняет только стенд этого Compose-проекта. Для сравнения с одной processing unit вручную установите `postgres.pool_max: 24` и согласуйте Compose, Router и Prometheus; автоматически бюджет пула не пересчитывается. При таком сравнении CPU вычислительного слоя уменьшается, поэтому это сравнение числа processing unit при фиксированном бюджете БД, а не равных суммарных CPU.

Генератор работает в открытой модели: constant — постоянный темп, ramp — от 20% до 100%, burst — тройной темп в интервале 40–60% прогона. По умолчанию 4096 документов, payload около 1 KiB, seed 42, warm-up 10 секунд. Hot направляет 80% итераций в partition 0, остальные — равномерно по ключам.

`ReadPercent` — доля логических read-only итераций. Update-итерация выполняет get и условный update: фактических HTTP-запросов больше, чем заданных итераций. 409 учитываются отдельно от технических ошибок. Пропуски генератора отражаются в `bench_dropped_iterations_total`; прогоны с заметными пропусками не доказывают насыщение сервера.

`db-lock` удерживает SHARE-lock таблицы 20 секунд в измеряемой фазе. Чтения продолжаются, записи ждут или завершаются по timeout. Это воспроизводимая демонстрация **блокировок БД**, а не имитация дискового или CPU-насыщения.

Результаты: `results/<UTC timestamp>/` — параметры, сырые HTTP-наблюдения, summary, метрики генератора, временные ряды Prometheus, параметры Docker и ссылка на дашборд с выбранным интервалом. Итоговые перцентили рассчитаны по завершённым HTTP-запросам измеряемой фазы, включая ошибки; они не включают незапущенные итерации. Для сравнения используйте одинаковые данные и настройки, несколько повторов, проверяйте запас ресурсов генератора/мониторинга. Локальные измерения не являются production SLO.
