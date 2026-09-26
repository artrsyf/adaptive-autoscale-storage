package api

import (
	document "autoscale-distr-storage/processing-unit/internal/document/domain"
	"autoscale-distr-storage/processing-unit/internal/processingunit/service"
	"context"
)

// DocumentCommandService — порт прикладных команд для входного HTTP-адаптера.
type DocumentCommandService interface {
	// Execute — Выполняет команду приложения с отменой через context.
	Execute(context.Context, document.Command, service.ProcessingUnitExecution) (document.Record, error)
}
