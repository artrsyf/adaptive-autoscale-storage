package dto

import (
	document "autoscale-distr-storage/processing-unit/internal/document/domain"
	"encoding/json"
)

// DocumentCreateRequest содержит только поля команды create.
type DocumentCreateRequest struct {
	PartitionKey string          `json:"partition_key"`
	ID           string          `json:"id"`
	Payload      json.RawMessage `json:"payload"`
}

// Command преобразует DTO в команду create для бизнес-валидации сервисом.
func (documentCreateRequest DocumentCreateRequest) Command() document.Command {
	return document.Command{Operation: "create", Key: document.Key{PartitionKey: documentCreateRequest.PartitionKey, ID: documentCreateRequest.ID}, Payload: document.Payload(documentCreateRequest.Payload)}
}
