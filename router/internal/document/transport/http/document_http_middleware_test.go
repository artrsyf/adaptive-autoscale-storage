package api

import (
	"autoscale-distr-storage/router/internal/assignment/domain"
	document "autoscale-distr-storage/router/internal/document/domain"
	"context"
	"github.com/prometheus/client_golang/prometheus"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestDocumentRoutes проверяет реальные маршруты, формы DTO, общие ограничения и статусы метрик.
func TestDocumentRoutes(test *testing.T) {
	for _, testCase := range []struct {
		method, path, body, operation string
		status, calls                 int
	}{
		{"POST", "/create", `{"partition_key":"a","id":"b","payload":{"x":1}}`, "create", 201, 1},
		{"POST", "/get", `{"partition_key":"a","id":"b"}`, "get", 200, 1},
		{"POST", "/update", `{"partition_key":"a","id":"b","payload":{"x":2},"expected_revision":"v0"}`, "update", 200, 1},
		{"POST", "/delete", `{"partition_key":"a","id":"b","expected_revision":"v0"}`, "delete", 200, 1},
		{"POST", "/get", `{"partition_key":"a","id":"b","payload":{}}`, "get", 400, 0},
		{"POST", "/create", `{"partition_key":"a","id":"b","payload":{},"expected_revision":"v0"}`, "create", 400, 0},
		{"POST", "/delete", `{"partition_key":"a","id":"b","expected_revision":"v0","payload":{}}`, "delete", 400, 0},
		{"GET", "/get", "", "get", 405, 0},
		{"POST", "/missing", "", "unknown", 404, 0},
		{"POST", "/get/extra", "", "unknown", 404, 0},
		{"POST", "/get", `{"partition_key":"a","id":"b"} {}`, "get", 400, 0},
		{"POST", "/get", `{"partition_key":"` + strings.Repeat("x", 600) + `","id":"b"}`, "get", 413, 0},
		{"POST", "/get", `{"partition_key":"a","id":"b"}` + strings.Repeat(" ", 600), "get", 413, 0},
	} {
		test.Run(testCase.method+testCase.path+testCase.body, func(test *testing.T) {
			calls := 0
			registry := prometheus.NewRegistry()
			port := commandServiceFunc(func(operationContext context.Context, documentCommand document.Command) (document.Record, assignment.AssignmentRoute, error) {
				calls++
				if documentCommand.Operation != testCase.operation || documentCommand.Key.ID != "b" {
					test.Fatal("wrong command")
				}
				deadline, hasDeadline := operationContext.Deadline()
				if !hasDeadline || time.Until(deadline) > time.Second {
					test.Fatal("shared deadline missing")
				}
				return document.Record{Key: documentCommand.Key, Payload: documentCommand.Payload, Revision: "v1"}, assignment.AssignmentRoute{Partition: 42, Epoch: "7", Node: assignment.AssignmentNode{ID: "node-1"}}, nil
			})
			documentHttpRoutes := NewDocumentHttpRoutes(DocumentHttpConfig{MaxBodyBytes: 512, RequestTimeout: 1000000000}, port, NewDocumentHttpMetrics(registry, "test", "node-1"))
			httpRequest := httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body))
			httpRequest.Header.Set("X-Partition-ID", "42")
			httpRequest.Header.Set("X-PU-ID", "node-1")
			httpRequest.Header.Set("X-Assignment-Epoch", "7")
			responseRecorder := httptest.NewRecorder()
			documentHttpRoutes.ServeHTTP(responseRecorder, httpRequest)
			if responseRecorder.Code != testCase.status || calls != testCase.calls {
				test.Fatalf("status=%d calls=%d body=%s", responseRecorder.Code, calls, responseRecorder.Body)
			}
			if testCase.status == 405 && responseRecorder.Header().Get("Allow") != "POST" {
				test.Fatal("Allow missing")
			}
			if testCase.operation == "delete" && testCase.status == 200 && strings.Contains(responseRecorder.Body.String(), `"payload"`) {
				test.Fatal("delete exposes payload")
			}
			families, err := registry.Gather()
			if err != nil {
				test.Fatal(err)
			}
			total := 0.0
			for _, family := range families {
				if family.GetName() == "storage_requests_total" {
					for _, metric := range family.Metric {
						if metric.GetCounter().GetValue() > 0 {
							total += metric.GetCounter().GetValue()
							for _, label := range metric.Label {
								if label.GetName() == "operation" && label.GetValue() != testCase.operation {
									test.Fatal("wrong operation label")
								}
							}
						}
					}
				}
			}
			if total != 1 {
				test.Fatalf("requests counted %v times", total)
			}
		})
	}
}

// TestSharedDocumentDeadline проверяет отмену сервисного вызова и единый ответ 504 на каждом endpoint.
func TestSharedDocumentDeadline(test *testing.T) {
	for _, operation := range []string{"create", "get", "update", "delete"} {
		test.Run(operation, func(test *testing.T) {
			port := commandServiceFunc(func(operationContext context.Context, documentCommand document.Command) (document.Record, assignment.AssignmentRoute, error) {
				<-operationContext.Done()
				return document.Record{}, assignment.AssignmentRoute{}, operationContext.Err()
			})
			documentHttpRoutes := NewDocumentHttpRoutes(DocumentHttpConfig{MaxBodyBytes: 1024, RequestTimeout: 1000000}, port, NewDocumentHttpMetrics(prometheus.NewRegistry(), "test", "node-1"))
			httpRequest := httptest.NewRequest("POST", "/"+operation, strings.NewReader(`{"partition_key":"a","id":"b"}`))
			httpRequest.Header.Set("X-Partition-ID", "42")
			httpRequest.Header.Set("X-PU-ID", "node-1")
			httpRequest.Header.Set("X-Assignment-Epoch", "7")
			responseRecorder := httptest.NewRecorder()
			documentHttpRoutes.ServeHTTP(responseRecorder, httpRequest)
			if responseRecorder.Code != 504 || !strings.Contains(responseRecorder.Body.String(), "deadline_exceeded") {
				test.Fatalf("status=%d body=%s", responseRecorder.Code, responseRecorder.Body)
			}
		})
	}
}
