# Эпик 3: статус валидации

## Выполнено 27.09.2026

- `make check` — PASS для processing unit, Router, Control Plane, workload client и partitioning harness;
- `make test-race` — PASS для всех пяти Go-модулей;
- unit-тесты проверяют полноту snapshot, монотонность epoch, клиент Control Plane, HTTP-контракт, Router snapshot replacement и перенос fencing token;
- PostgreSQL integration-тесты добавлены для чтения активного snapshot, CAS-конфликта, stale epoch и конфликтующих `FOR SHARE`/`FOR UPDATE` locks;
- `docker compose config --quiet` — PASS; локальный Docker config выдал предупреждение о недоступном пользовательском `config.json`, не относящееся к Compose-файлу;
- Grafana dashboard JSON успешно разобран PowerShell `ConvertFrom-Json`;
- `git diff --check` — PASS;
- `003_assignment.sql` успешно применена к существующему PostgreSQL volume: создано состояние epoch 1, зарегистрированы три processing unit и 128 назначений;
- `make test-all` — PASS, включая контейнерные race/integration-тесты, сборку и запуск полного Compose stack и публичный CRUD smoke через Router;
- через Control Plane опубликован epoch 2 с тем же распределением; Router без перезапуска подхватил snapshot и вернул `X-Assignment-Epoch: 2` в публичном CRUD-запросе;
- прямой запрос к processing unit со stale epoch 1 отклонён с `409 wrong_assignment`;
- после публикации epoch 2 повторно выполнены `make check` и `make integration` — PASS; PostgreSQL integration-тесты читают активный fence динамически и не зависят от seed epoch.

## Дополнительная эксплуатационная проверка

Длинная нагрузочная серия не входит в функциональную приёмку fencing. Отдельный диагностический прогон нужен для оценки накладных расходов общей строки `assignment_state`, времени ожидания locks и визуальной проверки новых панелей Grafana под нагрузкой.

## Перенос Control Plane на Scala — 11.10.2026

- `sbt -batch test` — PASS: 8 тестов для domain, service, строгих zio-json DTO, YAML-конфига и repository-контракта;
- `docker compose --profile test run --build --rm control-plane-tests` — PASS с настоящей Compose PostgreSQL: чтение активного snapshot и stale CAS;
- `sbt -batch scalafmtCheckAll` — PASS;
- `sbt -batch stage` — PASS для native-packager runtime-дистрибутива;
- `docker compose config --quiet` — PASS;
- после перезапуска Docker daemon `docker compose up -d --build --wait --remove-orphans` — PASS: Scala runtime-образ собран, Control Plane и зависимые сервисы healthy;
- live HTTP-проверка — PASS: `/livez`, `/readyz`, `/metrics`, `GET /assignments/current`, `400 invalid_request` для malformed/unknown JSON, `409 epoch_conflict` для stale CAS и `413` для тела больше 1 MiB;
- `benchmarks/smoke.ps1` — PASS: публичный CRUD, revision conflicts и delete/recreate через Router после замены Control Plane.
