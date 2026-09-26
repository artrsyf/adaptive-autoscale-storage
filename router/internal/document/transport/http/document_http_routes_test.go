package api

import (
	"autoscale-distr-storage/router/internal/assignment/domain"
	document "autoscale-distr-storage/router/internal/document/domain"
	"context"
	"errors"
	"github.com/prometheus/client_golang/prometheus"
	"net/http/httptest"
	"strings"
	"testing"
)

type commandServiceFunc func(context.Context, document.Command) (document.Record, assignment.AssignmentRoute, error)

// Execute вызывает подставленный тестовый сервис с полученными аргументами.
func (executeCommand commandServiceFunc) Execute(operationContext context.Context, documentCommand document.Command) (document.Record, assignment.AssignmentRoute, error) {
	return executeCommand(operationContext, documentCommand)
}

// TestHTTPBoundary проверяет преобразование DTO, публичные ошибки и метаданные ответа HTTP.
func TestHTTPBoundary(test *testing.T) {
	for _, testCase := range []struct {
		name, body string
		err        error
		status     int
		calls      int
	}{
		{"valid", `{"partition_key":"a","id":"b"}`, nil, 200, 1},
		{"unknown field", `{"partition_key":"a","id":"b","sql":"drop"}`, nil, 400, 0},
		{"multiple JSON", `{"partition_key":"a","id":"b"} {}`, nil, 400, 0},
		{"invalid entity", `{"id":"b"}`, document.ErrInvalid, 400, 1},
		{"not found", `{"partition_key":"a","id":"b"}`, document.ErrNotFound, 404, 1},
		{"wrong owner", `{"partition_key":"a","id":"b"}`, document.ErrWrongAssignment, 409, 1},
		{"overload", `{"partition_key":"a","id":"b"}`, document.ErrOverloaded, 429, 1},
		{"processing unit failure", `{"partition_key":"a","id":"b"}`, document.ErrProcessingUnitUnavailable, 502, 1},
		{"database failure", `{"partition_key":"a","id":"b"}`, errors.New("private database URL"), 503, 1},
		{"timeout", `{"partition_key":"a","id":"b"}`, context.DeadlineExceeded, 504, 1},
	} {
		test.Run(testCase.name, func(test *testing.T) {
			calls := 0
			service := commandServiceFunc(func(operationContext context.Context, documentCommand document.Command) (document.Record, assignment.AssignmentRoute, error) {
				calls++
				if documentCommand.Operation != "get" || documentCommand.Key.ID != "b" {
					test.Fatal("DTO not mapped to service arguments")
				}
				return document.Record{Key: documentCommand.Key}, assignment.AssignmentRoute{Partition: 42, Epoch: "9", Node: assignment.AssignmentNode{ID: "processing-unit-2"}}, testCase.err
			})
			documentHttpRoutes := NewDocumentHttpRoutes(DocumentHttpConfig{MaxBodyBytes: 1024, RequestTimeout: 1000000000}, service, NewDocumentHttpMetrics(prometheus.NewRegistry(), "router", "router"))
			httpRequest := httptest.NewRequest("POST", "/get", strings.NewReader(testCase.body))
			httpRequest.Header.Set("X-Assignment-Epoch", "7")
			responseRecorder := httptest.NewRecorder()
			documentHttpRoutes.ServeHTTP(responseRecorder, httpRequest)
			if responseRecorder.Code != testCase.status || calls != testCase.calls {
				test.Fatalf("status %d calls %d body %s", responseRecorder.Code, calls, responseRecorder.Body)
			}
			if strings.Contains(responseRecorder.Body.String(), "private") {
				test.Fatal("internal error exposed")
			}
			if responseRecorder.Code == 200 && (responseRecorder.Header().Get("X-PU-ID") != "processing-unit-2" || responseRecorder.Header().Get("X-Partition-ID") != "42" || responseRecorder.Header().Get("X-Assignment-Epoch") != "9") {
				test.Fatal("service result metadata not mapped to HTTP headers")
			}
		})
	}
}
