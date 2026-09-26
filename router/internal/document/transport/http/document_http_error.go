package api

import (
	document "autoscale-distr-storage/router/internal/document/domain"
	"autoscale-distr-storage/router/internal/document/transport/dto"
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

// documentHttpError задаёт публичную ошибку декодирования HTTP-запроса.
type documentHttpError struct {
	status int
	code   string
}

// Error возвращает публичный код ошибки.
func (httpError documentHttpError) Error() string { return httpError.code }

// writeDocumentError записывает статус и JSON-код без внутренних подробностей.
func writeDocumentError(responseWriter http.ResponseWriter, status int, code string) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(status)
	_ = json.NewEncoder(responseWriter).Encode(dto.DocumentErrorResponse{Error: code})
}

// writeDocumentFailure переводит транспортные и доменные ошибки в единый HTTP-контракт.
func writeDocumentFailure(responseWriter http.ResponseWriter, err error) {
	var requestError documentHttpError
	if errors.As(err, &requestError) {
		writeDocumentError(responseWriter, requestError.status, requestError.code)
		return
	}
	switch {
	case errors.Is(err, document.ErrInvalid):
		writeDocumentError(responseWriter, 400, "invalid_request")
	case errors.Is(err, document.ErrNotFound):
		writeDocumentError(responseWriter, 404, "not_found")
	case errors.Is(err, document.ErrConflict):
		writeDocumentError(responseWriter, 409, "revision_conflict")
	case errors.Is(err, document.ErrExists):
		writeDocumentError(responseWriter, 409, "already_exists")
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		writeDocumentError(responseWriter, 504, "deadline_exceeded")
	case errors.Is(err, document.ErrWrongAssignment):
		writeDocumentError(responseWriter, 409, "wrong_assignment")
	case errors.Is(err, document.ErrCommandTooLarge):
		writeDocumentError(responseWriter, 413, "body_too_large")
	case errors.Is(err, document.ErrOverloaded):
		writeDocumentError(responseWriter, 429, "overloaded")
	case errors.Is(err, document.ErrProcessingUnitUnavailable):
		writeDocumentError(responseWriter, 502, "pu_unavailable")
	case errors.Is(err, document.ErrInvalidProcessingUnitResponse):
		writeDocumentError(responseWriter, 502, "invalid_upstream_response")
	default:
		writeDocumentError(responseWriter, 503, "database_unavailable")
	}
}
