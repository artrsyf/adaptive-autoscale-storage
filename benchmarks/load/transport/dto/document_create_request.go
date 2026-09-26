package dto

import "encoding/json"

// DocumentCreateRequest содержит параметры HTTP-команды create нагрузочного клиента.
type DocumentCreateRequest struct {
	PartitionKey string          `json:"partition_key"`
	ID           string          `json:"id"`
	Payload      json.RawMessage `json:"payload"`
}
