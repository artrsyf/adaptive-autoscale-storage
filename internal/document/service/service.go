// Package service implements document use cases without infrastructure dependencies.
package service

import (
	"context"

	document "autoscale-distr-storage/internal/document/domain"
)

// Repository describes persistence guarantees required by the use cases.
// Create rejects duplicates. Replace/Delete atomically check the expected
// revision and mutate, returning ErrNotFound or ErrConflict as appropriate.
// Writes return success only after commit; revisions cannot be reused after
// deletion and recreation. Implementations must respect context cancellation.
type Repository interface {
	Create(context.Context, document.Key, document.Payload) (document.Record, error)
	Get(context.Context, document.Key) (document.Record, error)
	Replace(context.Context, document.Key, document.Payload, string) (document.Record, error)
	Delete(context.Context, document.Key, string) (document.Record, error)
}

type Service struct{ repository Repository }

func New(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) Create(ctx context.Context, key document.Key, payload document.Payload) (document.Record, error) {
	if err := key.Validate(); err != nil {
		return document.Record{}, err
	}
	if err := payload.Validate(); err != nil {
		return document.Record{}, err
	}
	return s.repository.Create(ctx, key, payload)
}

func (s *Service) Get(ctx context.Context, key document.Key) (document.Record, error) {
	if err := key.Validate(); err != nil {
		return document.Record{}, err
	}
	return s.repository.Get(ctx, key)
}

func (s *Service) Update(ctx context.Context, key document.Key, payload document.Payload, expectedRevision string) (document.Record, error) {
	if err := key.Validate(); err != nil {
		return document.Record{}, err
	}
	if err := payload.Validate(); err != nil {
		return document.Record{}, err
	}
	if err := document.ValidateRevision(expectedRevision); err != nil {
		return document.Record{}, err
	}
	return s.repository.Replace(ctx, key, payload, expectedRevision)
}

func (s *Service) Delete(ctx context.Context, key document.Key, expectedRevision string) (document.Record, error) {
	if err := key.Validate(); err != nil {
		return document.Record{}, err
	}
	if err := document.ValidateRevision(expectedRevision); err != nil {
		return document.Record{}, err
	}
	return s.repository.Delete(ctx, key, expectedRevision)
}
