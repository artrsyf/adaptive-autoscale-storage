package httpclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"autoscale-distr-storage/router/internal/assignment/domain"
	document "autoscale-distr-storage/router/internal/document/domain"
)

// TestSendToSelectedDestinationWithoutRetry проверяет отправку команды и метаданных выбранному адресату ровно один раз.
func TestSendToSelectedDestinationWithoutRetry(test *testing.T) {
	count := 0
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, httpRequest *http.Request) {
		count++
		if httpRequest.Header.Get("X-Assignment-Epoch") != "7" || httpRequest.URL.Path != "/update" || httpRequest.Header.Get("X-Partition-ID") != "106" || httpRequest.Header.Get("X-PU-ID") != "chosen" {
			test.Error("wrong request metadata")
		}
		var body map[string]any
		if err := json.NewDecoder(httpRequest.Body).Decode(&body); err != nil || body["partition_key"] != "abc" {
			test.Error("command not encoded")
		}
		responseWriter.WriteHeader(409)
		responseWriter.Write([]byte(`{"error":"revision_conflict"}`))
	}))
	defer server.Close()
	processingUnitHttpClient := &ProcessingUnitHttpClient{Client: server.Client()}
	_, err := processingUnitHttpClient.Send(context.Background(), assignment.AssignmentRoute{Partition: 106, Node: assignment.AssignmentNode{ID: "chosen", URL: server.URL}, Epoch: "7"}, document.Command{Operation: "update", Key: document.Key{PartitionKey: "abc", ID: "x"}})
	if !errors.Is(err, document.ErrConflict) || count != 1 {
		test.Fatalf("calls=%d err=%v", count, err)
	}
}

// TestUnexpectedResponseBecomesDomainError проверяет, что неизвестный ответ исполнителя не раскрывает внутренние подробности.
func TestUnexpectedResponseBecomesDomainError(test *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, httpRequest *http.Request) {
		responseWriter.WriteHeader(500)
		responseWriter.Write([]byte("private internal text"))
	}))
	defer server.Close()
	processingUnitHttpClient := &ProcessingUnitHttpClient{Client: server.Client()}
	_, err := processingUnitHttpClient.Send(context.Background(), assignment.AssignmentRoute{Node: assignment.AssignmentNode{URL: server.URL}}, document.Command{Operation: "get"})
	if !errors.Is(err, document.ErrInvalidProcessingUnitResponse) {
		test.Fatal(err)
	}
}

// TestProcessingUnitErrorTranslation проверяет таблицу перевода ошибок HTTP в ошибки Router.
func TestProcessingUnitErrorTranslation(test *testing.T) {
	for _, testCase := range []struct {
		status int
		code   string
		want   error
	}{
		{400, "invalid_request", document.ErrInvalid}, {404, "not_found", document.ErrNotFound},
		{409, "revision_conflict", document.ErrConflict}, {409, "already_exists", document.ErrExists},
		{409, "wrong_assignment", document.ErrWrongAssignment}, {413, "body_too_large", document.ErrCommandTooLarge},
		{429, "overloaded", document.ErrOverloaded}, {503, "database_unavailable", document.ErrUnavailable},
		{504, "deadline_exceeded", context.DeadlineExceeded}, {500, "private_details", document.ErrInvalidProcessingUnitResponse},
	} {
		if !errors.Is(decodeError(testCase.status, testCase.code), testCase.want) {
			test.Fatalf("%d %s", testCase.status, testCase.code)
		}
	}
}
