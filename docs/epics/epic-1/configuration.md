# Конфигурация локального стенда

## Файлы и ответственность

| Файл | Что настраивает |
|---|---|
| `processing-unit/config.yaml` | HTTP processing unit, admission, PostgreSQL-адаптер, локальная epoch и число разделов |
| `router/config.yaml` | HTTP Router, admission, исходящий processing unit-клиент, assignment |
| `docker-compose.yaml` | Сервисы, DNS-имена, порты, образы, ресурсы, параметры сервера PostgreSQL |
| `deploy/prometheus/prometheus.yaml` | Scrape targets и интервалы |
| `deploy/grafana/provisioning/` | Datasource и загрузка дашборда |
| `deploy/postgres-exporter/queries.yaml` | Дополнительные SQL-метрики БД |
| `benchmarks/load/config.yaml` | Параметры генератора нагрузки |
| `.env` | Только `POSTGRES_PASSWORD` и `MONITOR_PASSWORD` |

Router и processing unit — отдельные бинарники и образы, переключателя роли нет. `router/cmd/router` читает `router/config.yaml`, `processing-unit/cmd/processing-unit` — `processing-unit/config.yaml`; идентификатор processing unit задаётся `--node`. Файлы читаются напрямую своими потребителями. Генератора, промежуточного каталога и шаблонов нет. Загрузчик YAML находится в `internal/utils` каждого приложения; типы `DocumentPostgresConfig`, `DocumentHttpConfig` и другие — в модулях использования. Неизвестные поля, повторные ключи, несколько YAML-документов и неверные длительности отклоняются. Длительности записываются как `2500ms`, `3s`, `1m`. Include, подстановка переменных и слияние конфигов приложения не поддерживаются; другой файл передаётся через `--config`.

Все пути и команды ниже указаны относительно корня репозитория.

## Запуск и применение изменений

```powershell
# Только при первой настройке, не перезаписывайте существующие пароли:
Copy-Item .env.example .env
docker compose up -d --build --wait
./benchmarks/smoke.ps1
docker compose --profile test run --build --rm tests
docker compose --profile test run --build --rm router-tests
```

После изменения приложения или подключённых YAML-файлов выполните `docker compose up -d --build --force-recreate --wait`. Можно пересоздать только затронутые сервисы. Обычный `up` не отслеживает содержимое bind-mounted файлов; автоматической перезагрузки нет. `docker compose config --quiet` проверяет Compose, но не согласованность всех адресов. Readiness, smoke-check и Prometheus Targets показывают фактические ошибки связности.

## Допустимые дубли и их цена

- Имя, порт и база PostgreSQL должны совпадать в Compose, `processing-unit/config.yaml` и подключении exporter.
- Имена/порты processing unit должны совпадать в Compose, `router.assignment.nodes` и Prometheus. Processing unit не хранит адреса соседей. `processing.epoch` и `processing.partition_count` должны совпадать с `router.assignment.epoch` и `router.assignment.partitions`. При изменении числа нод обновляются Compose, Router и Prometheus; суммарные пулы должны оставлять запас в PostgreSQL `max_connections`.
- Порт Router повторяется в workload-конфиге; внешний адрес smoke-check меняется через `-Api`. Адреса мониторинга в скрипте эксперимента задаются через `-Prometheus` и `-Grafana`.
- Имя Compose-проекта используется фильтрами контейнеров в дашборде; при переименовании обновите фильтры.

Эти небольшие повторы заменяют собственный слой генерации инфраструктуры. Настройки нагрузки переопределяются флагами клиента; `benchmarks/run.ps1` передаёт их через API CLI и архивирует фактический `config.json`, Compose без подстановки паролей, оба конфига приложений и метрики.

## PostgreSQL и наблюдаемость

`processing-unit/migrations/001_documents.sql` создаёт таблицу; `002_monitoring.sql` включает `pg_stat_statements` и создаёт пользователя мониторинга с `pg_monitor`. Файл `.sql` запускает `psql` через штатный entrypoint образа PostgreSQL; `\getenv` получает пароли из окружения. Дополнительная shell-обёртка не нужна. Скрипты выполняются только при первом создании volume. Изменение пароля в `.env` не меняет пароль существующей БД; её данные не удаляются автоматически.

`deploy/postgres-exporter/queries.yaml` содержит четыре читаемых агрегатных запроса к системным представлениям: состояния/ожидания соединений, блокируемые сессии, статистика SQL и WAL. Это конфигурация exporter. Имена таблиц приложения в ней не используются. SQL-статистика охватывает всю базу, включая мониторинг; WAL относится ко всему экземпляру PostgreSQL. Цена такой независимости от схемы — отсутствие отдельной статистики только CRUD-запросов. Дашборд отражает эту семантику; для отдельных операций остаются метрики приложения.

Исходящий клиент Router настраивается секцией `processing_unit_client`. Идентификаторы и DNS исполнителей: `processing-unit-1`, `processing-unit-2`, `processing-unit-3`; Prometheus job и role: `processing-unit`. Старые временные ряды с другими labels остаются историческими данными.

Настройки `api.max_body_bytes` и `api.request_timeout` общие для create/get/update/delete внутри каждого приложения; конфигов на отдельные endpoint нет. Middleware задаёт deadline контексту до декодирования. Отмена кооперативная: чтение из сети дополнительно ограничено `server.read_timeout`, запись — `server.write_timeout`.

## Проверка сохранённого дашборда

После нагрузочного прогона генератор удаляется. Для проверки его панелей используйте сохранённый интервал, а не текущее время:

```powershell
$env:DASHBOARD_URL = Get-Content results/<run>/dashboard-url.txt
node benchmarks/check-dashboard.cjs results/dashboard-check
Remove-Item Env:DASHBOARD_URL
```

Нужны Node.js, Playwright и установленный Edge; NODE_PATH может указывать на уже установленный Playwright. Проверка выполняет 41 PromQL-запрос и сохраняет снимки пяти разделов. Пустые результаты перечисляются отдельно, не заменяются нулями. Скриншоты всё равно нужно просмотреть: успешный запрос не доказывает читаемость графика.
