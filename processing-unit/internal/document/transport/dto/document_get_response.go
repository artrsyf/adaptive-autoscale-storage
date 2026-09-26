package dto

import (
	document "autoscale-distr-storage/processing-unit/internal/document/domain"
	"encoding/json"
	"time"
)

// DocumentGetResponse описывает успешный ответ get.
type DocumentGetResponse struct {
	PartitionKey string          `json:"partition_key"`
	ID           string          `json:"id"`
	Revision     string          `json:"revision"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	Payload      json.RawMessage `json:"payload,omitempty"`
}

// NewDocumentGetResponse преобразует результат операции в DTO ответа.
func NewDocumentGetResponse(documentRecord document.Record) DocumentGetResponse {
	return DocumentGetResponse{PartitionKey: documentRecord.PartitionKey, ID: documentRecord.ID, Revision: documentRecord.Revision, CreatedAt: documentRecord.CreatedAt, UpdatedAt: documentRecord.UpdatedAt, Payload: json.RawMessage(documentRecord.Payload)}
}
