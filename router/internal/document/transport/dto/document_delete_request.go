package dto

import (
	document "autoscale-distr-storage/router/internal/document/domain"
)

// DocumentDeleteRequest содержит только поля команды delete.
type DocumentDeleteRequest struct {
	PartitionKey     string `json:"partition_key"`
	ID               string `json:"id"`
	ExpectedRevision string `json:"expected_revision"`
}

// Command преобразует DTO в команду delete для бизнес-валидации сервисом.
func (documentDeleteRequest DocumentDeleteRequest) Command() document.Command {
	return document.Command{Operation: "delete", Key: document.Key{PartitionKey: documentDeleteRequest.PartitionKey, ID: documentDeleteRequest.ID}, ExpectedRevision: documentDeleteRequest.ExpectedRevision}
}
