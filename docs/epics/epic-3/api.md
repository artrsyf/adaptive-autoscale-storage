# Assignment API

Control Plane обслуживает административный HTTP API. В Compose он доступен другим сервисам как `http://control-plane:8080` и привязан только к loopback хоста как `http://localhost:8081`; это не пользовательский CRUD API.

## Получение активного snapshot

```http
GET /assignments/current
```

Успех — `200` и полный snapshot:

```json
{
  "epoch": 1,
  "partition_count": 128,
  "nodes": [
    {"id": "processing-unit-1", "url": "http://processing-unit-1:8080"}
  ],
  "assignments": [
    {"partition": 0, "node_id": "processing-unit-1"}
  ]
}
```

Фактический массив `assignments` содержит ровно `partition_count` элементов. Router устанавливает snapshot только после полной локальной проверки.

## Публикация

```http
PUT /assignments/current
Content-Type: application/json

{
  "expected_epoch": 1,
  "assignment": {
    "epoch": 2,
    "partition_count": 128,
    "nodes": [...],
    "assignments": [...]
  }
}
```

Публикация требует `assignment.epoch = expected_epoch + 1`. `partition_count` должен совпадать с активным snapshot. Control Plane проверяет уникальность нод, HTTP origins, полноту разделов и ссылки на владельцев до открытия транзакции.

Ответы:

| Статус | Код | Значение |
|---|---|---|
| 200 | — | snapshot атомарно опубликован |
| 400 | `invalid_request` / `invalid_assignment` | неверный JSON, переход epoch или неполная карта |
| 413 | — | тело запроса превышает настроенный лимит |
| 409 | `epoch_conflict` | активный epoch не равен `expected_epoch` |
| 503 | `assignment_unavailable` | PostgreSQL недоступен или публикация не завершилась |

Повтор публикации с тем же `expected_epoch` получает конфликт и не изменяет активную карту. Аутентификация административного API не входит в локальный исследовательский стенд; host-порт доступен только через loopback и не должен публиковаться наружу.
