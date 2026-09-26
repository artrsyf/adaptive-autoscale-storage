package dto

import (
	document "autoscale-distr-storage/processing-unit/internal/document/domain"
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
