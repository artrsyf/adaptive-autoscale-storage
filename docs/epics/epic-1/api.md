# Document API v1

Router and processing unit are independent applications communicating over HTTP/JSON. Their Go models and adapters are private. Router remains a data-plane service; a future control plane will manage topology and assignments.

## Commands

All operations use POST. `/create` accepts `partition_key`, `id`, and object `payload`; `/get` accepts the two key fields; `/update` adds `payload` and `expected_revision`; `/delete` adds `expected_revision`. Keys are nonempty UTF-8 strings of at most 256 bytes. Unknown command fields are rejected against the operation-specific DTO. Fields belonging to a different operation are rejected as 400 invalid_json, even if empty or null (for example payload in /get or expected_revision in /create). Update replaces the entire payload. Revision is opaque to clients.

Create returns 201; get/update/delete return 200. Responses contain document key and revision; delete omits payload. Errors use `{"error":"code"}`. Statuses: 400 invalid_request, 404 not_found, 409 already_exists/revision_conflict/wrong_assignment, 413 body_too_large, 429 overloaded, 502 pu_unavailable/invalid_upstream_response, 503 database_unavailable, 504 deadline_exceeded. Writes are not retried automatically.

## Routing

Hash the UTF-8 partition key with SHA-256. Interpret the first eight bytes as an unsigned big-endian integer and take modulo partition count. With 128 partitions, `abc` maps to partition 106. Select the owner by partition modulo the number of nodes, preserving configured node order. Only Router stores node order and URLs. Processing unit stores matching partition count and epoch in its local processing configuration.

Router sends `X-Assignment-Epoch`, `X-Partition-ID` and `X-PU-ID` from the selected route, overriding any external client metadata. Processing unit checks epoch, destination identity and partition range before persistence. It trusts Router to map keys to partitions and owners; it has no topology or hash implementation. These checks are neither authentication nor ownership fencing. This internal request contract requires Router and processing unit to be deployed together after the change. Successful responses carry `X-Assignment-Epoch`, `X-Partition-ID`, and `X-PU-ID`. The abbreviated header `X-PU-ID` and error `pu_unavailable` remain v1 wire names; both refer to a processing unit. Deployment names and internal Go identifiers use the full name.

## Compatibility checks

Router tests the fixed routing example; processing unit tests execution metadata and admission independently. Each module tests its own transport. `benchmarks/smoke.ps1` exercises CRUD and revision semantics through the running Router and processing unit without importing either application. Changes to fields, statuses, routing or headers must remain compatible across both adapters and clients.
