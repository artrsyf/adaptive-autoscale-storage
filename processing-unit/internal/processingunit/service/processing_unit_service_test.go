package service

import (
	"context"
	"errors"
	"testing"

	document "autoscale-distr-storage/processing-unit/internal/document/domain"
	documents "autoscale-distr-storage/processing-unit/internal/document/service"
)

type repository struct {
	get func(context.Context, document.Key) (document.Record, error)
}

// Get передаёт обращение тестовой функции хранилища.
func (documentRepository repository) Get(operationContext context.Context, documentKey document.Key) (document.Record, error) {
	return documentRepository.get(operationContext, documentKey)
}

// Create передаёт обращение тестовой функции хранилища.
func (documentRepository repository) Create(operationContext context.Context, documentKey document.Key, _ document.Payload) (document.Record, error) {
	return documentRepository.get(operationContext, documentKey)
}

// Replace передаёт обращение тестовой функции хранилища.
func (documentRepository repository) Replace(operationContext context.Context, documentKey document.Key, _ document.Payload, _ string) (document.Record, error) {
	return documentRepository.get(operationContext, documentKey)
}

// Delete передаёт обращение тестовой функции хранилища.
func (documentRepository repository) Delete(operationContext context.Context, documentKey document.Key, _ string) (document.Record, error) {
	return documentRepository.get(operationContext, documentKey)
}

type observer struct{ partitions []int }

// RecordPartitionRequest сохраняет номер раздела для проверки вызовов наблюдателя.
func (partitionObserver *observer) RecordPartitionRequest(partitionID int) {
	partitionObserver.partitions = append(partitionObserver.partitions, partitionID)
}

// TestExecutionValidationBeforePersistence проверяет отклонение неверных адресата, epoch и раздела до обращения к хранилищу.
func TestExecutionValidationBeforePersistence(test *testing.T) {
	calls := 0
	documentRepository := repository{get: func(_ context.Context, documentKey document.Key) (document.Record, error) {
		calls++
		return document.Record{Key: documentKey}, nil
	}}
	metrics := &observer{}
	processingUnitService, err := NewProcessingUnitService(ProcessingUnitServiceConfig{MaxInflight: 1, PartitionCount: 128, Epoch: "7"}, "node-1", documents.NewDocumentService(documentRepository), metrics)
	if err != nil {
		test.Fatal(err)
	}
	documentCommand := document.Command{Operation: "get", Key: document.Key{PartitionKey: "abc", ID: "x"}}
	for _, execution := range []ProcessingUnitExecution{{106, "node-1", ""}, {106, "node-1", "old"}, {106, "other", "7"}, {-1, "node-1", "7"}, {128, "node-1", "7"}} {
		if _, err := processingUnitService.Execute(context.Background(), documentCommand, execution); !errors.Is(err, document.ErrWrongAssignment) {
			test.Fatalf("execution=%v err=%v", execution, err)
		}
	}
	if calls != 0 || len(metrics.partitions) != 0 {
		test.Fatal("invalid execution reached persistence or metrics")
	}
	record, err := processingUnitService.Execute(context.Background(), documentCommand, ProcessingUnitExecution{106, "node-1", "7"})
	if err != nil || record.Key != documentCommand.Key || calls != 1 || len(metrics.partitions) != 1 {
		test.Fatal("valid execution failed")
	}
}

// TestAdmissionAndCancellation проверяет лимит выполняемых команд, учёт разделов и передачу отмены репозиторию.
func TestAdmissionAndCancellation(test *testing.T) {
	entered := make(chan struct{})
	metrics := &observer{}
	documentRepository := repository{get: func(operationContext context.Context, _ document.Key) (document.Record, error) {
		close(entered)
		<-operationContext.Done()
		return document.Record{}, operationContext.Err()
	}}
	processingUnitService, _ := NewProcessingUnitService(ProcessingUnitServiceConfig{MaxInflight: 1, PartitionCount: 128, Epoch: "7"}, "node-1", documents.NewDocumentService(documentRepository), metrics)
	documentCommand := document.Command{Operation: "get", Key: document.Key{PartitionKey: "abc", ID: "x"}}
	execution := ProcessingUnitExecution{106, "node-1", "7"}
	operationContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := processingUnitService.Execute(operationContext, documentCommand, execution)
		done <- err
	}()
	<-entered
	if _, err := processingUnitService.Execute(context.Background(), documentCommand, execution); !errors.Is(err, document.ErrOverloaded) {
		test.Fatal(err)
	}
	cancel()
	if !errors.Is(<-done, context.Canceled) || len(metrics.partitions) != 2 {
		test.Fatal("admission metrics or cancellation incorrect")
	}
}
