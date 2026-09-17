# Autoscale Distributed Storage

Локальный исследовательский стенд: **HTTP Router → Go PU → PostgreSQL**, с документным API, статическими логическими разделами и наблюдаемостью. Архитектура и границы эпика: [docs/first-epic-contract.md](docs/first-epic-contract.md).

## Структура кода

- `cmd/storage` — запуск; `internal/app` — сборка зависимостей и жизненный цикл.
- `internal/document/domain` — модель и инварианты; `service` — сценарии CRUD и интерфейс репозитория.
- `internal/document/repository/postgres` — SQL, транзакции, пул; `transport/httpapi` — DTO, хендлеры и forwarding.
- `internal/assignment` — статическое распределение; `internal/platform/telemetry` — метрики.
- `benchmarks/load` — отдельный Go-модуль и образ клиента; `deploy/Dockerfile.test` — тестовый образ.

Приложение не запускает тесты или нагрузку. Направление зависимостей и пример update описаны в [архитектуре кода](docs/code-architecture.md).

## Запуск

Нужны Docker Desktop с WSL2/Linux containers и Docker Compose. Для локальной разработки — Go 1.25.7 (toolchain указан в `go.mod`). Команды выполняются из корня проекта в PowerShell 7.

```powershell
Copy-Item .env.example .env
docker compose up -d --build --wait
```

`.env.example` включает три PU. Не перезаписывайте существующий `.env`, если меняли настройки.

- API: http://localhost:8080
- Grafana: http://localhost:3000/d/storage-baseline
- Prometheus: http://localhost:9090
- Readiness: http://localhost:8080/readyz

Grafana открывается с правами просмотра; datasource и дашборд загружаются автоматически. Пустые панели до первого прогона ожидаемы. Отсутствующие измерения не подменяются нулями.

Внешние порты привязаны к loopback. Пароли в Compose — публичные значения только для локального стенда; не используйте их вне него. cAdvisor имеет привилегии и read-only mounts для чтения ресурсов Docker VM. Метрики контейнеров относятся к Linux VM, а не ко всей Windows-машине.

```powershell
docker compose logs --tail 100 router pu-1 postgres
docker compose down
```

Остановка сохраняет данные PostgreSQL и Prometheus. `down -v` удаляет их. SQL из `migrations/` применяется при инициализации нового PostgreSQL volume; это пока начальная схема, не универсальный migration runner.

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
| 409 | Ключ занят, версия устарела или неверное назначение PU |
| 429 | Исчерпан лимит активных запросов |
| 502 / 503 | PU / PostgreSQL недоступны |
| 504 | Истёк deadline; результат записи может быть неопределённым |

Update полностью заменяет payload. Update/delete проверяют revision под блокировкой строки в транзакции. Новая UUID revision создаётся при каждом create/update; повторное создание не переиспользует старую версию. Ответ отправляется после commit. Автоматических повторов записи и восстановления потерянного ответа нет.

## Маршрутизация и ограничения

Partition = первые 8 байт SHA-256 от UTF-8 `partition_key`, unsigned big-endian, modulo 128. Владелец = `partition % len(nodes)` в порядке `NODES`. Router и PU используют одинаковую карту и `ASSIGNMENT_EPOCH`. Это статическая конфигурация: изменение топологии требует остановки нагрузки и согласованного перезапуска компонентов. Проверка epoch не является динамическим fencing.

Все PU работают с общей таблицей PostgreSQL. SQL-планов, поиска по JSON, физического шардирования, Kafka, Redis, control plane и autoscaling в этом эпике нет.

Начальные лимиты: PostgreSQL 1 CPU / 768 MiB, PU 0.5 CPU / 192 MiB каждый. Пул: 8 соединений на PU, суммарно 24. Параметры доступны в Compose и `.env`. Readiness проверяет доступность таблицы; перегрузка ограничивается числом активных запросов, скрытой неограниченной очереди нет.

## Проверки

```powershell
go test ./...
go vet ./...
go -C benchmarks/load test ./...
go -C benchmarks/load vet ./...
docker compose --profile test run --build --rm tests
./benchmarks/smoke.ps1
```

Команда Compose выполняет тесты с race detector и настоящим PostgreSQL. Без `TEST_DATABASE_URL` локальные интеграционные тесты пропускаются. Проверяются конфликтующие обновления, duplicate create, delete/recreate, отмена и таймаут под блокировкой, маршрутизация и отказ при перегрузке. `smoke.ps1` проверяет публичный API через Router. Вариант `./benchmarks/smoke.ps1 -FailureChecks` дополнительно останавливает и поднимает PostgreSQL, проверяя ошибки, readiness и сохранность записи; запускайте его отдельно от нагрузки.

## Нагрузочные эксперименты

Генератор собирается отдельно из `benchmarks/load` и обращается к HTTP API. Корневой `go test ./...` не включает этот вложенный модуль. `run.ps1` пересобирает образ генератора перед прогоном.

```powershell
./benchmarks/run.ps1 -Scenario constant -Distribution uniform -Rate 100 -Seconds 60
./benchmarks/run.ps1 -Scenario constant -Distribution hot -Rate 100 -Seconds 60
./benchmarks/run.ps1 -Scenario ramp -Rate 500 -Seconds 90
./benchmarks/run.ps1 -Scenario burst -Rate 100 -Seconds 90
./benchmarks/run.ps1 -Scenario db-lock -Rate 100 -Seconds 60 -ReadPercent 20
./benchmarks/run.ps1 -Nodes 1 -Rate 100 -Seconds 60
```

Запускайте эксперименты последовательно. Скрипт меняет только стенд этого Compose-проекта. При одной PU суммарный пул остаётся 24; CPU вычислительного слоя уменьшается, поэтому это сравнение числа PU при фиксированном бюджете БД, а не равных суммарных CPU.

Генератор работает в открытой модели: constant — постоянный темп, ramp — от 20% до 100%, burst — тройной темп в интервале 40–60% прогона. По умолчанию 4096 документов, payload около 1 KiB, seed 42, warm-up 10 секунд. Hot направляет 80% итераций в partition 0, остальные — равномерно по ключам.

`ReadPercent` — доля логических read-only итераций. Update-итерация выполняет get и условный update: фактических HTTP-запросов больше, чем заданных итераций. 409 учитываются отдельно от технических ошибок. Пропуски генератора отражаются в `bench_dropped_iterations_total`; прогоны с заметными пропусками не доказывают насыщение сервера.

`db-lock` удерживает SHARE-lock таблицы 20 секунд в измеряемой фазе. Чтения продолжаются, записи ждут или завершаются по timeout. Это воспроизводимая демонстрация **блокировок БД**, а не имитация дискового или CPU-насыщения.

Результаты: `results/<UTC timestamp>/` — параметры, сырые HTTP-наблюдения, summary, метрики генератора, временные ряды Prometheus, параметры Docker и ссылка на дашборд с выбранным интервалом. Итоговые перцентили рассчитаны по завершённым HTTP-запросам измеряемой фазы, включая ошибки; они не включают незапущенные итерации. Для сравнения используйте одинаковые данные и настройки, несколько повторов, проверяйте запас ресурсов генератора/мониторинга. Локальные измерения не являются production SLO.
