// Package service owns destination selection and routing admission, without HTTP or storage dependencies.
package service

import (
	"context"
	"fmt"

	"autoscale-distr-storage/router/internal/assignment/domain"
	document "autoscale-distr-storage/router/internal/document/domain"
)

// DocumentRoutingService валидирует документную команду, выбирает исполнителя и ограничивает число запросов.

type DocumentRoutingService struct {
	assignment AssignmentResolver
	client     ProcessingUnitClient
	slots      chan struct{}
}

// NewDocumentRoutingService создаёт сервис выбора исполнителя с ограниченным числом одновременных запросов.
func NewDocumentRoutingService(documentRoutingConfig DocumentRoutingConfig, assignmentResolver AssignmentResolver, client ProcessingUnitClient) (*DocumentRoutingService, error) {
	if documentRoutingConfig.MaxInflight < 1 {
		return nil, fmt.Errorf("invalid Router routing configuration")
	}
	return &DocumentRoutingService{assignment: assignmentResolver, client: client, slots: make(chan struct{}, documentRoutingConfig.MaxInflight)}, nil
}

// Execute проверяет команду, выбирает владельца и отправляет запрос без повторов с ограничением параллелизма.
func (documentRoutingService *DocumentRoutingService) Execute(operationContext context.Context, documentCommand document.Command) (document.Record, assignment.AssignmentRoute, error) {
	if err := documentCommand.Validate(); err != nil {
		return document.Record{}, assignment.AssignmentRoute{}, err
	}
	route := documentRoutingService.assignment.Resolve(documentCommand.Key.PartitionKey)
	select {
	case documentRoutingService.slots <- struct{}{}:
		defer func() { <-documentRoutingService.slots }()
	default:
		return document.Record{}, route, document.ErrOverloaded
	}
	record, err := documentRoutingService.client.Send(operationContext, route, documentCommand)
	return record, route, err
}
