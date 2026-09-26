// Package service owns processing unit command admission, execution metadata checks and document execution.
package service

import (
	"context"
	"fmt"

	document "autoscale-distr-storage/processing-unit/internal/document/domain"
	documents "autoscale-distr-storage/processing-unit/internal/document/service"
)

// ProcessingUnitService проверяет метаданные исполнения и admission перед вызовом DocumentService.
type ProcessingUnitService struct {
	nodeID    string
	config    ProcessingUnitServiceConfig
	documents *documents.DocumentService
	observer  PartitionRequestObserver
	slots     chan struct{}
}

// NewProcessingUnitService проверяет локальную конфигурацию и создаёт ограничитель числа выполняемых команд.
func NewProcessingUnitService(processingUnitServiceConfig ProcessingUnitServiceConfig, nodeID string, documentService *documents.DocumentService, partitionRequestObserver PartitionRequestObserver) (*ProcessingUnitService, error) {
	if processingUnitServiceConfig.Validate() != nil || nodeID == "" {
		return nil, fmt.Errorf("invalid processing unit processing configuration")
	}
	return &ProcessingUnitService{nodeID: nodeID, config: processingUnitServiceConfig, documents: documentService, observer: partitionRequestObserver, slots: make(chan struct{}, processingUnitServiceConfig.MaxInflight)}, nil
}

// Execute проверяет команду и метаданные исполнения, ограничивает параллелизм и вызывает CRUD-сервис.
func (processingUnitService *ProcessingUnitService) Execute(operationContext context.Context, documentCommand document.Command, execution ProcessingUnitExecution) (document.Record, error) {
	var empty document.Record
	if err := documentCommand.Validate(); err != nil {
		return empty, err
	}
	if execution.Epoch != processingUnitService.config.Epoch || execution.NodeID != processingUnitService.nodeID || execution.Partition < 0 || execution.Partition >= processingUnitService.config.PartitionCount {
		return empty, document.ErrWrongAssignment
	}
	processingUnitService.observer.RecordPartitionRequest(execution.Partition)
	select {
	case processingUnitService.slots <- struct{}{}:
		defer func() { <-processingUnitService.slots }()
	default:
		return empty, document.ErrOverloaded
	}
	var record document.Record
	var err error
	switch documentCommand.Operation {
	case "create":
		record, err = processingUnitService.documents.Create(operationContext, documentCommand.Key, documentCommand.Payload)
	case "get":
		record, err = processingUnitService.documents.Get(operationContext, documentCommand.Key)
	case "update":
		record, err = processingUnitService.documents.Update(operationContext, documentCommand.Key, documentCommand.Payload, documentCommand.ExpectedRevision)
	case "delete":
		record, err = processingUnitService.documents.Delete(operationContext, documentCommand.Key, documentCommand.ExpectedRevision)
	}
	return record, err
}
