package service

import (
	"autoscale-distr-storage/router/internal/assignment/domain"
	document "autoscale-distr-storage/router/internal/document/domain"
	"context"
)

// ProcessingUnitClient — порт отправки документной команды уже выбранному исполнителю.
type ProcessingUnitClient interface {
	// Send — Отправляет команду на уже выбранную ноду без автоматических повторов.
	Send(context.Context, assignment.AssignmentRoute, document.Command) (document.Record, error)
}
