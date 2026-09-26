# Эпик 2: поставка и точка продолжения

## Статус

Рабочий эпик завершён 26.09.2026 в ветке `epic/2-partitioning-research`, база — `master` на `4be05de`. Runtime стенда не изменён: Router и три processing unit используют прежнюю статическую конфигурацию.

## Состав поставки

- Добавлен независимый Go-модуль `benchmarks/partitioning` без импортов приложений и без запуска из server startup.
- Все стратегии реализуют общий `DistributionAlgorithm`; runner и расчёт метрик не зависят от modulo, hash ring или logical assignment.
- Реализованы modulo, consistent hashing с настраиваемыми virtual nodes и fixed logical partitions с детерминированным минимизирующим движение assignment.
- Канонические переходы: `3 → 4`, `3 → 5`, `5 → 3`, `5 → 10`; 1 000 000 ключей, 128 logical partitions, seed 42.
- Сохраняются конфигурация, окружение, сырые повторения, JSON summary, два CSV, Markdown и пять SVG-графиков.
- Timing выполняется пакетами не менее 50 ms, чтобы короткие операции не терялись на разрешении системного таймера.
- `make build`, `test`, `test-race`, `vet`, `fmt` и `check` включают четвёртый Go-модуль; `make partitioning` запускает полный автономный эксперимент.
- Добавлена `.gitattributes` с LF для Go-файлов и Makefile, чтобы `fmt-check` не зависел от Windows checkout.
- Спецификация, методика, численные результаты, сгруппированные графики и архитектурное решение находятся в документах эпика.

## Исследовательский вывод

Для дальнейшего runtime выбран **fixed logical partitions + explicit assignment**. В канонической серии он отличается от consistent hashing по moved keys менее чем на 0.42 процентного пункта, но даёт более ровную нагрузку и явную управляемую единицу для epoch, fencing и migration state machine.

Modulo сохраняется как простой контрольный вариант, consistent hashing — как исследовательский baseline. Выбор не означает, что физическая миграция, динамическая topology или fencing уже реализованы.

Полные данные, графики, анализ и ограничения: [results.md](results.md).

## Выполненная валидация

- `make check` — PASS для processing unit, Router, workload client и partitioning harness.
- `go -C benchmarks/partitioning test -race ./...` — PASS.
- `make partitioning` — PASS на полной канонической конфигурации.
- Создание JSONL, JSON, CSV, Markdown и SVG проверено фактическим прогоном.
- Сгруппированные moved-keys и CV charts визуально проверены; SVG валидны как XML.
- `git diff --check` — PASS.

В локальном агентском окружении Go-команды выполнялись с `GOFLAGS=-buildvcs=false`, поскольку Git отклонял VCS stamping из-за несовпадения владельца рабочего каталога. Это ограничение окружения, не исходного кода. Docker не запускался: benchmark автономный и не требует WSL2 или стенда.

Локальные полные архивы находятся в игнорируемом `results/partitioning/` и не входят в Git. Репрезентативные числа и SVG зафиксированы в `results.md` и `assets/`.

## Следующая агентская сессия

Рекомендуемый новый рабочий эпик — **управляемый assignment и fencing**: завершение A4 до начала динамической топологии.

Перед реализацией нужно оформить `docs/epics/epic-3/README.md` и определить:

1. модель неизменяемого `PartitionAssignment` и монотонного epoch;
2. авторитетный источник и атомарную публикацию нового assignment;
3. способ доставки согласованного snapshot Router и processing unit;
4. PostgreSQL-backed fencing, запрещающий stale write старого владельца, а не только локальное сравнение строки epoch;
5. поведение чтений и записей во время смены epoch;
6. восстановление после частично применённого обновления;
7. тесты гонок старого и нового владельцев и критерии безопасного перехода.

Не следует начинать с автозапуска/остановки processing unit, Kubernetes или физического переноса строк. Сначала должен существовать безопасный протокол ownership. Выбранный в этом эпике allocator затем станет источником desired assignment для такого протокола.

Отдельное необязательное усиление исследования — sensitivity series по нескольким seed, `partitionCount = 64/128/256/512` и `virtualNodes = 16/64/128/256`. Оно не блокирует runtime A4 и может выполняться независимо.

## Команды для продолжения

```powershell
make check
make partitioning
go -C benchmarks/partitioning test -race ./...
```

Перед любым Docker-запуском пользователь должен подтвердить доступность WSL2. Merge в `master` выполняется только после отдельного review и явного разрешения пользователя.
