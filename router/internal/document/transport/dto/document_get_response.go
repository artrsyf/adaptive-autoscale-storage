package dto

import (
	document "autoscale-distr-storage/router/internal/document/domain"
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

// Record преобразует ответ исполнителя в локальную модель Router.
func (documentGetResponse DocumentGetResponse) Record() document.Record {
	return document.Record{Key: document.Key{PartitionKey: documentGetResponse.PartitionKey, ID: documentGetResponse.ID}, Revision: documentGetResponse.Revision, CreatedAt: documentGetResponse.CreatedAt, UpdatedAt: documentGetResponse.UpdatedAt, Payload: document.Payload(documentGetResponse.Payload)}
}
