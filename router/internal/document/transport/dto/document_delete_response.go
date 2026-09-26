package dto

import (
	document "autoscale-distr-storage/router/internal/document/domain"
	"time"
)

// DocumentDeleteResponse описывает успешный ответ delete.
type DocumentDeleteResponse struct {
	PartitionKey string    `json:"partition_key"`
	ID           string    `json:"id"`
	Revision     string    `json:"revision"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// NewDocumentDeleteResponse преобразует результат операции в DTO ответа.
func NewDocumentDeleteResponse(documentRecord document.Record) DocumentDeleteResponse {
	return DocumentDeleteResponse{PartitionKey: documentRecord.PartitionKey, ID: documentRecord.ID, Revision: documentRecord.Revision, CreatedAt: documentRecord.CreatedAt, UpdatedAt: documentRecord.UpdatedAt}
}

// Record преобразует ответ исполнителя в локальную модель Router.
func (documentDeleteResponse DocumentDeleteResponse) Record() document.Record {
	return document.Record{Key: document.Key{PartitionKey: documentDeleteResponse.PartitionKey, ID: documentDeleteResponse.ID}, Revision: documentDeleteResponse.Revision, CreatedAt: documentDeleteResponse.CreatedAt, UpdatedAt: documentDeleteResponse.UpdatedAt}
}
