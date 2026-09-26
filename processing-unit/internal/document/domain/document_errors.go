package document

import "errors"

var (
	ErrNotFound        = errors.New("not_found")
	ErrConflict        = errors.New("revision_conflict")
	ErrExists          = errors.New("already_exists")
	ErrInvalid         = errors.New("invalid_request")
	ErrWrongAssignment = errors.New("wrong_assignment")
	ErrOverloaded      = errors.New("overloaded")
	ErrUnavailable     = errors.New("database_unavailable")
)
