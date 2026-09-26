package api

import (
	"autoscale-distr-storage/router/internal/assignment/domain"
	document "autoscale-distr-storage/router/internal/document/domain"
	"context"
)

// DocumentCommandService — порт прикладных команд для входного HTTP-адаптера.
type DocumentCommandService interface {
	// Execute — Выполняет команду приложения с отменой через context.
	Execute(context.Context, document.Command) (document.Record, assignment.AssignmentRoute, error)
}
