package dto

import (
	document "autoscale-distr-storage/processing-unit/internal/document/domain"
	"encoding/json"
)

// DocumentUpdateRequest содержит только поля команды update.
type DocumentUpdateRequest struct {
	PartitionKey     string          `json:"partition_key"`
	ID               string          `json:"id"`
	Payload          json.RawMessage `json:"payload"`
	ExpectedRevision string          `json:"expected_revision"`
}

// Command преобразует DTO в команду update для бизнес-валидации сервисом.
func (documentUpdateRequest DocumentUpdateRequest) Command() document.Command {
	return document.Command{Operation: "update", Key: document.Key{PartitionKey: documentUpdateRequest.PartitionKey, ID: documentUpdateRequest.ID}, Payload: document.Payload(documentUpdateRequest.Payload), ExpectedRevision: documentUpdateRequest.ExpectedRevision}
}
