package dto

import (
	document "autoscale-distr-storage/processing-unit/internal/document/domain"
)

// DocumentGetRequest содержит только поля команды get.
type DocumentGetRequest struct {
	PartitionKey string `json:"partition_key"`
	ID           string `json:"id"`
}

// Command преобразует DTO в команду get для бизнес-валидации сервисом.
func (documentGetRequest DocumentGetRequest) Command() document.Command {
	return document.Command{Operation: "get", Key: document.Key{PartitionKey: documentGetRequest.PartitionKey, ID: documentGetRequest.ID}}
}
