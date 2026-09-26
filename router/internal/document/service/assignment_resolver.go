package service

import "autoscale-distr-storage/router/internal/assignment/domain"

type AssignmentResolver interface {
	// Resolve возвращает раздел, владельца и epoch для ключа документа.
	Resolve(string) assignment.AssignmentRoute
}
