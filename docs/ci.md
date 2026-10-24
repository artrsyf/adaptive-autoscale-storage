# Continuous Integration

Базовый GitHub Actions workflow находится в `.github/workflows/ci.yml` и запускается для каждого `push` и `pull_request`. Он только проверяет репозиторий: deployment, публикация image, доступ к k3s и repository secrets в workflow отсутствуют.

Проверки разделены по независимым job:

- `Go / <module>` — матрица четырёх Go-модулей с Go 1.25.7: `gofmt`, build, vet и unit tests;
- `Scala Control Plane` — Temurin JDK 21 и sbt 1.10.11 из `project/build.properties`: `scalafmtCheckAll`, compile и unit tests;
- `Helm chart` — Helm 3.21.0: lint и render обоих режимов chart, initial install и upgrade, со всеми каноническими файлами через `--set-file`.

Jobs используют только `contents: read`, checkout не сохраняет credentials. Повторный push в ту же ветку отменяет устаревший незавершённый run. Зависимости Go и sbt кэшируются штатными setup actions.

Docker integration, Compose smoke, k3s rollout, нагрузочные сценарии и визуальная проверка Grafana намеренно не входят в базовый CI: им нужны stateful services или локальный кластер. Перед инфраструктурным релизом они выполняются отдельно по процедурам соответствующего эпика; локальная полная регрессия остаётся `make test-all`.
