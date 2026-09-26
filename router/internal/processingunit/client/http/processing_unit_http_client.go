package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"autoscale-distr-storage/router/internal/assignment/domain"
	document "autoscale-distr-storage/router/internal/document/domain"
	"autoscale-distr-storage/router/internal/document/transport/dto"
)

// HTTPClient is the outbound transport dependency consumed by the forwarding adapter.
type HTTPClient interface {
	// Do — Выполняет HTTP-запрос, соблюдая его context.
	Do(*http.Request) (*http.Response, error)
}

// ProcessingUnitHttpClient implements the routing service's outgoing port; it never selects a node.
type ProcessingUnitHttpClient struct{ Client HTTPClient }

// Send отправляет команду на уже выбранную ноду с метаданными исполнения; автоматических повторов нет.
func (processingUnitHttpClient *ProcessingUnitHttpClient) Send(operationContext context.Context, route assignment.AssignmentRoute, documentCommand document.Command) (document.Record, error) {
	var empty document.Record
	data, err := json.Marshal(documentRequest(documentCommand))
	if err != nil {
		return empty, err
	}
	httpRequest, err := http.NewRequestWithContext(operationContext, http.MethodPost, route.Node.URL+"/"+documentCommand.Operation, bytes.NewReader(data))
	if err != nil {
		return empty, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("X-Assignment-Epoch", route.Epoch)
	httpRequest.Header.Set("X-Partition-ID", strconv.Itoa(route.Partition))
	httpRequest.Header.Set("X-PU-ID", route.Node.ID)
	httpResponse, err := processingUnitHttpClient.Client.Do(httpRequest)
	if err != nil {
		if operationContext.Err() != nil {
			return empty, operationContext.Err()
		}
		return empty, document.ErrProcessingUnitUnavailable
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode >= 400 {
		var documentErrorResponse dto.DocumentErrorResponse
		if json.NewDecoder(io.LimitReader(httpResponse.Body, 4096)).Decode(&documentErrorResponse) != nil {
			return empty, document.ErrInvalidProcessingUnitResponse
		}
		return empty, decodeError(httpResponse.StatusCode, documentErrorResponse.Error)
	}
	record, err := decodeDocumentRecord(json.NewDecoder(io.LimitReader(httpResponse.Body, 2<<20)), documentCommand.Operation)
	if err != nil {
		return empty, document.ErrInvalidProcessingUnitResponse
	}
	return record, nil
}

// decodeError переводит известную пару HTTP-статуса и кода ошибки в локальную ошибку Router.
func decodeError(status int, code string) error {
	switch {
	case status == 400 && (code == "invalid_request" || code == "invalid_json"):
		return document.ErrInvalid
	case status == 404 && code == "not_found":
		return document.ErrNotFound
	case status == 409 && code == "revision_conflict":
		return document.ErrConflict
	case status == 409 && code == "already_exists":
		return document.ErrExists
	case status == 409 && code == "wrong_assignment":
		return document.ErrWrongAssignment
	case status == 413 && code == "body_too_large":
		return document.ErrCommandTooLarge
	case status == 429 && code == "overloaded":
		return document.ErrOverloaded
	case status == 503 && code == "database_unavailable":
		return document.ErrUnavailable
	case status == 504 && code == "deadline_exceeded":
		return context.DeadlineExceeded
	default:
		return document.ErrInvalidProcessingUnitResponse
	}
}

// Ready последовательно проверяет /readyz всех настроенных исполнителей в рамках переданного контекста.
func (processingUnitHttpClient *ProcessingUnitHttpClient) Ready(operationContext context.Context, nodes []assignment.AssignmentNode) error {
	for _, assignmentNode := range nodes {
		httpRequest, err := http.NewRequestWithContext(operationContext, http.MethodGet, assignmentNode.URL+"/readyz", nil)
		if err != nil {
			return err
		}
		httpResponse, err := processingUnitHttpClient.Client.Do(httpRequest)
		if err != nil {
			return err
		}
		httpResponse.Body.Close()
		if httpResponse.StatusCode != http.StatusOK {
			return document.ErrProcessingUnitUnavailable
		}
	}
	return nil
}
