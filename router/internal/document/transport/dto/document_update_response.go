package dto

import (
	document "autoscale-distr-storage/router/internal/document/domain"
	"encoding/json"
	"time"
)

// DocumentUpdateResponse описывает успешный ответ update.
type DocumentUpdateResponse struct {
	PartitionKey string          `json:"partition_key"`
	ID           string          `json:"id"`
	Revision     string          `json:"revision"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	Payload      json.RawMessage `json:"payload,omitempty"`
}

// NewDocumentUpdateResponse преобразует результат операции в DTO ответа.
func NewDocumentUpdateResponse(documentRecord document.Record) DocumentUpdateResponse {
	return DocumentUpdateResponse{PartitionKey: documentRecord.PartitionKey, ID: documentRecord.ID, Revision: documentRecord.Revision, CreatedAt: documentRecord.CreatedAt, UpdatedAt: documentRecord.UpdatedAt, Payload: json.RawMessage(documentRecord.Payload)}
}

// Record преобразует ответ исполнителя в локальную модель Router.
func (documentUpdateResponse DocumentUpdateResponse) Record() document.Record {
	return document.Record{Key: document.Key{PartitionKey: documentUpdateResponse.PartitionKey, ID: documentUpdateResponse.ID}, Revision: documentUpdateResponse.Revision, CreatedAt: documentUpdateResponse.CreatedAt, UpdatedAt: documentUpdateResponse.UpdatedAt, Payload: document.Payload(documentUpdateResponse.Payload)}
}
