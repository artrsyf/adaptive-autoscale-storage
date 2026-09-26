package dto

import "encoding/json"

// DocumentUpdateRequest содержит параметры HTTP-команды update нагрузочного клиента.
type DocumentUpdateRequest struct {
	PartitionKey     string          `json:"partition_key"`
	ID               string          `json:"id"`
	Payload          json.RawMessage `json:"payload"`
	ExpectedRevision string          `json:"expected_revision"`
}
