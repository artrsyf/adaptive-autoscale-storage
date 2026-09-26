package service

import (
	assignmentservice "autoscale-distr-storage/router/internal/assignment/service"
	"context"
	"errors"
	"testing"

	"autoscale-distr-storage/router/internal/assignment/domain"
	document "autoscale-distr-storage/router/internal/document/domain"
)

type clientFunc func(context.Context, assignment.AssignmentRoute, document.Command) (document.Record, error)

// Send вызывает тестовую реализацию исходящего клиента.
func (sendCommand clientFunc) Send(operationContext context.Context, assignmentRoute assignment.AssignmentRoute, documentCommand document.Command) (document.Record, error) {
	return sendCommand(operationContext, assignmentRoute, documentCommand)
}

// TestRoutingSelectsOwnerAndPropagatesError проверяет выбранный маршрут, передачу ошибки клиента и раннюю валидацию команды.
func TestRoutingSelectsOwnerAndPropagatesError(test *testing.T) {
	staticAssignmentService, _ := assignmentservice.NewStaticAssignmentService(assignmentservice.AssignmentConfig{Partitions: 128, Epoch: "7", Nodes: []assignment.AssignmentNode{{ID: "one", URL: "http://one:8080"}, {ID: "two", URL: "http://two:8080"}}})
	command := document.Command{Operation: "get", Key: document.Key{PartitionKey: "abc", ID: "x"}}
	expected := staticAssignmentService.Resolve("abc")
	calls := 0
	documentRoutingService, _ := NewDocumentRoutingService(DocumentRoutingConfig{MaxInflight: 1}, staticAssignmentService, clientFunc(func(operationContext context.Context, assignmentRoute assignment.AssignmentRoute, documentCommand document.Command) (document.Record, error) {
		calls++
		if assignmentRoute != expected || documentCommand.Key != command.Key || documentCommand.Operation != "get" {
			test.Fatal("wrong routing decision")
		}
		return document.Record{}, document.ErrConflict
	}))
	_, route, err := documentRoutingService.Execute(context.Background(), command)
	if route != expected || !errors.Is(err, document.ErrConflict) || calls != 1 {
		test.Fatalf("route=%v err=%v calls=%d", route, err, calls)
	}
	command.Key.ID = ""
	_, _, err = documentRoutingService.Execute(context.Background(), command)
	if !errors.Is(err, document.ErrInvalid) || calls != 1 {
		test.Fatal("invalid command reached processing unit")
	}
}

// TestRoutingAdmissionAndCancellation проверяет отказ при перегрузке Router и передачу отмены исходящему вызову.
func TestRoutingAdmissionAndCancellation(test *testing.T) {
	staticAssignmentService, _ := assignmentservice.NewStaticAssignmentService(assignmentservice.AssignmentConfig{Partitions: 128, Epoch: "1", Nodes: []assignment.AssignmentNode{{ID: "one", URL: "http://one:8080"}}})
	entered := make(chan struct{})
	documentRoutingService, _ := NewDocumentRoutingService(DocumentRoutingConfig{MaxInflight: 1}, staticAssignmentService, clientFunc(func(operationContext context.Context, _ assignment.AssignmentRoute, _ document.Command) (document.Record, error) {
		close(entered)
		<-operationContext.Done()
		return document.Record{}, operationContext.Err()
	}))
	command := document.Command{Operation: "get", Key: document.Key{PartitionKey: "a", ID: "b"}}
	operationContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, _, err := documentRoutingService.Execute(operationContext, command); done <- err }()
	<-entered
	_, _, err := documentRoutingService.Execute(context.Background(), command)
	if !errors.Is(err, document.ErrOverloaded) {
		test.Fatal(err)
	}
	cancel()
	if !errors.Is(<-done, context.Canceled) {
		test.Fatal("cancellation not propagated")
	}
}
