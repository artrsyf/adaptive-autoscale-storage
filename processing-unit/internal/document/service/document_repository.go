package service

import (
	document "autoscale-distr-storage/processing-unit/internal/document/domain"
	"context"
)

// DocumentRepository describes persistence guarantees required by the use cases.
// Create rejects duplicates. Replace/Delete atomically check the expected
// revision and mutate, returning ErrNotFound or ErrConflict as appropriate.
// Writes return success only after commit; revisions cannot be reused after
// deletion and recreation. Implementations must respect context cancellation.
type DocumentRepository interface {
	// Create — Создаёт запись атомарно; существующий ключ возвращает ErrExists.
	Create(context.Context, document.Key, document.Payload) (document.Record, error)
	// Get — Читает запись по ключу; отсутствие возвращает ErrNotFound.
	Get(context.Context, document.Key) (document.Record, error)
	// Replace — Атомарно проверяет revision и заменяет payload с новой версией.
	Replace(context.Context, document.Key, document.Payload, string) (document.Record, error)
	// Delete — Атомарно проверяет revision и удаляет запись.
	Delete(context.Context, document.Key, string) (document.Record, error)
}
