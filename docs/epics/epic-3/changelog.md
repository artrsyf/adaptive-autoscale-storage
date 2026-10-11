# Эпик 3: состав изменений и продолжение

## Реализованный срез

- добавлено независимое Scala 3/ZIO-приложение `control-plane` с первым функциональным модулем `assignment`, Tapir/zio-http transport и Doobie repository;
- PostgreSQL хранит versioned nodes и partition assignments, а singleton `assignment_state` атомарно указывает активный epoch;
- публикация проверяет полный snapshot и использует compare-and-swap ожидаемого epoch;
- Router заменил статическую конфигурацию на проверяемый атомарный snapshot, загружаемый при запуске и обновляемый в фоне;
- processing unit удалил локальные epoch и partition count;
- каждый CRUD выполняет PostgreSQL-backed fencing в одной транзакции с операцией документа;
- общий lock активного assignment задаёт порядок между длительной операцией и публикацией;
- Compose, Prometheus, Grafana, Makefile и документация расширены для Control Plane.

## Сохранённые границы

Control Plane не проксирует CRUD и не принимает scaling decisions. Router не подключён к PostgreSQL. Processing unit не хранит topology и не вычисляет hash partition. Физическая миграция строк, membership, Kubernetes, Kafka, Redis и autoscaling не добавлены.

## Следующая точка

Следующий рабочий инкремент зафиксирован как [эпик 4 — наблюдающий Control Plane v1](../epic-4/README.md). Он строит `SystemState` из существующих Prometheus-метрик и выдаёт advisory scaling decision, но не изменяет replicas и assignment.

Расчёт desired assignment и управление переходом topology переносятся на этап после Kubernetes baseline и появления actuator/reconciliation. До реализации физической migration state machine публикация допустима только при общей авторитетной PostgreSQL, где смена логического владельца не требует переноса строк; даже в этой модели scale-in требует отдельного порядка draining и fencing.
