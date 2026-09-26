package document

import "errors"

var (
	ErrNotFound                      = errors.New("not_found")
	ErrConflict                      = errors.New("revision_conflict")
	ErrExists                        = errors.New("already_exists")
	ErrInvalid                       = errors.New("invalid_request")
	ErrWrongAssignment               = errors.New("wrong_assignment")
	ErrOverloaded                    = errors.New("overloaded")
	ErrProcessingUnitUnavailable     = errors.New("pu_unavailable")
	ErrInvalidProcessingUnitResponse = errors.New("invalid_upstream_response")
	ErrUnavailable                   = errors.New("database_unavailable")
	ErrCommandTooLarge               = errors.New("body_too_large")
)
