// Package service implements document use cases without infrastructure dependencies.
package service

import (
	"context"

	document "autoscale-distr-storage/processing-unit/internal/document/domain"
)

// DocumentService реализует CRUD-сценарии и проверяет аргументы перед обращением к репозиторию.
type DocumentService struct{ repository DocumentRepository }

// NewDocumentService создаёт CRUD-сервис с репозиторием, обеспечивающим атомарные изменения.
func NewDocumentService(repository DocumentRepository) *DocumentService {
	return &DocumentService{repository: repository}
}

// Create проверяет ключ и payload и делегирует создание репозиторию.
func (documentService *DocumentService) Create(operationContext context.Context, key document.Key, payload document.Payload) (document.Record, error) {
	if err := key.Validate(); err != nil {
		return document.Record{}, err
	}
	if err := payload.Validate(); err != nil {
		return document.Record{}, err
	}
	return documentService.repository.Create(operationContext, key, payload)
}

// Get проверяет ключ и запрашивает документ у репозитория.
func (documentService *DocumentService) Get(operationContext context.Context, key document.Key) (document.Record, error) {
	if err := key.Validate(); err != nil {
		return document.Record{}, err
	}
	return documentService.repository.Get(operationContext, key)
}

// Update проверяет аргументы и делегирует атомарную замену payload с проверкой revision.
func (documentService *DocumentService) Update(operationContext context.Context, key document.Key, payload document.Payload, expectedRevision string) (document.Record, error) {
	if err := key.Validate(); err != nil {
		return document.Record{}, err
	}
	if err := payload.Validate(); err != nil {
		return document.Record{}, err
	}
	if err := document.ValidateRevision(expectedRevision); err != nil {
		return document.Record{}, err
	}
	return documentService.repository.Replace(operationContext, key, payload, expectedRevision)
}

// Delete проверяет ключ и revision и делегирует атомарное удаление репозиторию.
func (documentService *DocumentService) Delete(operationContext context.Context, key document.Key, expectedRevision string) (document.Record, error) {
	if err := key.Validate(); err != nil {
		return document.Record{}, err
	}
	if err := document.ValidateRevision(expectedRevision); err != nil {
		return document.Record{}, err
	}
	return documentService.repository.Delete(operationContext, key, expectedRevision)
}
