package dto

import (
	document "autoscale-distr-storage/router/internal/document/domain"
	"encoding/json"
	"time"
)

// DocumentCreateResponse описывает успешный ответ create.
type DocumentCreateResponse struct {
	PartitionKey string          `json:"partition_key"`
	ID           string          `json:"id"`
	Revision     string          `json:"revision"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	Payload      json.RawMessage `json:"payload,omitempty"`
}

// NewDocumentCreateResponse преобразует результат операции в DTO ответа.
func NewDocumentCreateResponse(documentRecord document.Record) DocumentCreateResponse {
	return DocumentCreateResponse{PartitionKey: documentRecord.PartitionKey, ID: documentRecord.ID, Revision: documentRecord.Revision, CreatedAt: documentRecord.CreatedAt, UpdatedAt: documentRecord.UpdatedAt, Payload: json.RawMessage(documentRecord.Payload)}
}

// Record преобразует ответ исполнителя в локальную модель Router.
func (documentCreateResponse DocumentCreateResponse) Record() document.Record {
	return document.Record{Key: document.Key{PartitionKey: documentCreateResponse.PartitionKey, ID: documentCreateResponse.ID}, Revision: documentCreateResponse.Revision, CreatedAt: documentCreateResponse.CreatedAt, UpdatedAt: documentCreateResponse.UpdatedAt, Payload: document.Payload(documentCreateResponse.Payload)}
}
