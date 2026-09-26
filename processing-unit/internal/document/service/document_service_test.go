package service

import (
	"context"
	"errors"
	"testing"

	document "autoscale-distr-storage/processing-unit/internal/document/domain"
)

type repositorySpy struct {
	calls            []string
	key              document.Key
	payload          document.Payload
	revision         string
	operationContext context.Context
	err              error
}

// capture записывает вызов и аргументы для проверки взаимодействия с репозиторием.
func (repositorySpy *repositorySpy) capture(operationContext context.Context, operation string, key document.Key, payload document.Payload, revision string) (document.Record, error) {
	repositorySpy.calls = append(repositorySpy.calls, operation)
	repositorySpy.operationContext, repositorySpy.key, repositorySpy.payload, repositorySpy.revision = operationContext, key, payload, revision
	return document.Record{Key: key, Revision: "new-revision"}, repositorySpy.err
}

// Create записывает вызов и аргументы для проверки взаимодействия с репозиторием.
func (repositorySpy *repositorySpy) Create(operationContext context.Context, documentKey document.Key, documentPayload document.Payload) (document.Record, error) {
	return repositorySpy.capture(operationContext, "create", documentKey, documentPayload, "")
}

// Get записывает вызов и аргументы для проверки взаимодействия с репозиторием.
func (repositorySpy *repositorySpy) Get(operationContext context.Context, documentKey document.Key) (document.Record, error) {
	return repositorySpy.capture(operationContext, "get", documentKey, nil, "")
}

// Replace записывает вызов и аргументы для проверки взаимодействия с репозиторием.
func (repositorySpy *repositorySpy) Replace(operationContext context.Context, documentKey document.Key, documentPayload document.Payload, expectedRevision string) (document.Record, error) {
	return repositorySpy.capture(operationContext, "replace", documentKey, documentPayload, expectedRevision)
}

// Delete записывает вызов и аргументы для проверки взаимодействия с репозиторием.
func (repositorySpy *repositorySpy) Delete(operationContext context.Context, documentKey document.Key, expectedRevision string) (document.Record, error) {
	return repositorySpy.capture(operationContext, "delete", documentKey, nil, expectedRevision)
}

// TestInvalidInputNeverReachesRepository проверяет, что некорректные аргументы CRUD не достигают репозитория.
func TestInvalidInputNeverReachesRepository(test *testing.T) {
	key := document.Key{PartitionKey: "tenant", ID: "doc"}
	for name, call := range map[string]func(*DocumentService) error{
		"create key": func(documentService *DocumentService) error {
			_, operationError := documentService.Create(context.Background(), document.Key{}, document.Payload(`{}`))
			return operationError
		},
		"create payload": func(documentService *DocumentService) error {
			_, operationError := documentService.Create(context.Background(), key, document.Payload(`[]`))
			return operationError
		},
		"get key": func(documentService *DocumentService) error {
			_, operationError := documentService.Get(context.Background(), document.Key{})
			return operationError
		},
		"update revision": func(documentService *DocumentService) error {
			_, operationError := documentService.Update(context.Background(), key, document.Payload(`{}`), "")
			return operationError
		},
		"update payload": func(documentService *DocumentService) error {
			_, operationError := documentService.Update(context.Background(), key, document.Payload(`null`), "v1")
			return operationError
		},
		"delete revision": func(documentService *DocumentService) error {
			_, operationError := documentService.Delete(context.Background(), key, "")
			return operationError
		},
	} {
		test.Run(name, func(test *testing.T) {
			repositorySpy := &repositorySpy{}
			if err := call(NewDocumentService(repositorySpy)); !errors.Is(err, document.ErrInvalid) {
				test.Fatalf("error: %v", err)
			}
			if len(repositorySpy.calls) != 0 {
				test.Fatalf("invalid input reached persistence: %v", repositorySpy.calls)
			}
		})
	}
}

// TestUpdateDelegatesAtomicCheckWithoutPreReadOrRetry проверяет передачу проверки revision репозиторию без предварительного чтения или повторов.
func TestUpdateDelegatesAtomicCheckWithoutPreReadOrRetry(test *testing.T) {
	key := document.Key{PartitionKey: "tenant", ID: "doc"}
	payload := document.Payload(`{"value":2}`)
	operationContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, failure := range []error{nil, document.ErrConflict, document.ErrNotFound, context.Canceled} {
		repositorySpy := &repositorySpy{err: failure}
		record, err := NewDocumentService(repositorySpy).Update(operationContext, key, payload, "v1")
		if !errors.Is(err, failure) {
			test.Fatalf("error lost: %v", err)
		}
		if len(repositorySpy.calls) != 1 || repositorySpy.calls[0] != "replace" || repositorySpy.operationContext != operationContext || repositorySpy.key != key || string(repositorySpy.payload) != string(payload) || repositorySpy.revision != "v1" {
			test.Fatalf("atomic contract changed: %+v", repositorySpy)
		}
		if err == nil && record.Revision != "new-revision" {
			test.Fatal("result lost")
		}
	}
}
