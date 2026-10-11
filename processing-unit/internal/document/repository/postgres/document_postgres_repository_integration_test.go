package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"autoscale-distr-storage/processing-unit/internal/document/domain"
	"autoscale-distr-storage/processing-unit/internal/utils"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"
)

// TestIntegrationDocumentLifecycle проверяет CRUD, конкуренцию, версии и отмену на PostgreSQL; без пароля пропускается.
func TestIntegrationDocumentLifecycle(test *testing.T) {
	password := os.Getenv("POSTGRES_PASSWORD")
	if password == "" {
		test.Skip("use docker compose --profile test run --build --rm tests for PostgreSQL integration tests")
	}
	var processingUnitConfig struct {
		Postgres   DocumentPostgresConfig `yaml:"postgres"`
		Server     map[string]any         `yaml:"server"`
		API        map[string]any         `yaml:"api"`
		Processing map[string]any         `yaml:"processing"`
	}
	err := utils.LoadYAML("../../../../config.yaml", &processingUnitConfig)
	if err != nil {
		test.Fatal(err)
	}
	processingUnitConfig.Postgres.PoolMax = 4
	operationContext, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	documentRepository, operationError := Open(operationContext, processingUnitConfig.Postgres, password, "integration", prometheus.NewRegistry())
	if operationError != nil {
		test.Fatal(operationError)
	}
	defer documentRepository.pool.Close()
	key := document.Key{PartitionKey: fmt.Sprintf("test-%d", time.Now().UnixNano()), ID: "doc"}
	payload := document.Payload(`{"value":1}`)
	fence := document.AssignmentFence{Partition: 0}
	if operationError = documentRepository.pool.QueryRow(operationContext, `SELECT assignment_state.epoch,partition_assignments.owner_node_id FROM assignment_state JOIN partition_assignments ON partition_assignments.epoch=assignment_state.epoch AND partition_assignments.partition_id=$1 WHERE assignment_state.singleton_id=true`, fence.Partition).Scan(&fence.Epoch, &fence.NodeID); operationError != nil {
		test.Fatal(operationError)
	}
	var expectedRevision string
	defer documentRepository.pool.Exec(context.Background(), `DELETE FROM documents WHERE partition_key=$1`, key.PartitionKey)
	first, operationError := documentRepository.Create(operationContext, key, payload, fence)
	if operationError != nil {
		test.Fatal(operationError)
	}
	staleFence := fence
	staleFence.Epoch++
	if _, operationError = documentRepository.Get(operationContext, key, staleFence); !errors.Is(operationError, document.ErrWrongAssignment) {
		test.Fatalf("stale assignment: %v", operationError)
	}

	// Publication takes an exclusive lock on assignment_state and must wait for an executing command's shared fence.
	fenceEntered := make(chan struct{})
	releaseFence := make(chan struct{})
	fenceDone := make(chan error, 1)
	go func() {
		connection, acquireError := documentRepository.pool.Acquire(operationContext)
		if acquireError != nil {
			fenceDone <- acquireError
			return
		}
		defer connection.Release()
		_, executeError := documentRepository.fenced(operationContext, connection, "test", fence, func(pgx.Tx) (document.Record, error) {
			close(fenceEntered)
			<-releaseFence
			return document.Record{}, nil
		})
		fenceDone <- executeError
	}()
	<-fenceEntered
	publishDone := make(chan error, 1)
	go func() {
		_, publishError := documentRepository.pool.Exec(operationContext, `UPDATE assignment_state SET updated_at=updated_at WHERE singleton_id=true`)
		publishDone <- publishError
	}()
	select {
	case publishError := <-publishDone:
		test.Fatalf("publication bypassed active fence: %v", publishError)
	case <-time.After(100 * time.Millisecond):
	}
	close(releaseFence)
	if operationError = <-fenceDone; operationError != nil {
		test.Fatal(operationError)
	}
	if operationError = <-publishDone; operationError != nil {
		test.Fatal(operationError)
	}
	if _, operationError = documentRepository.Create(operationContext, key, payload, fence); !errors.Is(operationError, document.ErrExists) {
		test.Fatalf("duplicate: %v", operationError)
	}
	expectedRevision = first.Revision
	var waitGroup sync.WaitGroup
	results := make(chan error, 8)
	for index := 0; index < 8; index++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			_, operationError := documentRepository.Replace(operationContext, key, payload, expectedRevision, fence)
			results <- operationError
		}()
	}
	waitGroup.Wait()
	close(results)
	wins, conflicts := 0, 0
	for operationError := range results {
		if operationError == nil {
			wins++
		} else if errors.Is(operationError, document.ErrConflict) {
			conflicts++
		} else {
			test.Fatal(operationError)
		}
	}
	if wins != 1 || conflicts != 7 {
		test.Fatalf("wins=%d conflicts=%d", wins, conflicts)
	}

	current, operationError := documentRepository.Get(operationContext, key, fence)
	if operationError != nil {
		test.Fatal(operationError)
	}
	deleteRevision := current.Revision
	if _, operationError = documentRepository.Delete(operationContext, key, deleteRevision, fence); operationError != nil {
		test.Fatal(operationError)
	}
	expectedRevision = ""
	recreated, operationError := documentRepository.Create(operationContext, key, payload, fence)
	if operationError != nil {
		test.Fatal(operationError)
	}
	if recreated.Revision == first.Revision || recreated.Revision == current.Revision {
		test.Fatal("revision reused")
	}
	if _, operationError = documentRepository.Delete(operationContext, key, deleteRevision, fence); !errors.Is(operationError, document.ErrConflict) {
		test.Fatalf("stale delete: %v", operationError)
	}
	cancelled, cancelNow := context.WithCancel(operationContext)
	cancelNow()
	if _, operationError = documentRepository.Get(cancelled, key, fence); !errors.Is(operationError, context.Canceled) {
		test.Fatalf("cancellation: %v", operationError)
	}
	// A real row lock must time out without changing the record.
	lock, operationError := documentRepository.pool.Begin(operationContext)
	if operationError != nil {
		test.Fatal(operationError)
	}
	if _, operationError = lock.Exec(operationContext, `SELECT 1 FROM documents WHERE partition_key=$1 AND id=$2 FOR UPDATE`, key.PartitionKey, key.ID); operationError != nil {
		test.Fatal(operationError)
	}
	short, stop := context.WithTimeout(operationContext, 100*time.Millisecond)
	expectedRevision = recreated.Revision
	_, operationError = documentRepository.Replace(short, key, payload, expectedRevision, fence)
	stop()
	_ = lock.Rollback(operationContext)
	if !errors.Is(operationError, context.DeadlineExceeded) {
		test.Fatalf("locked update: %v", operationError)
	}
	after, operationError := documentRepository.Get(operationContext, key, fence)
	if operationError != nil || after.Revision != recreated.Revision {
		test.Fatalf("timeout changed record: %v", operationError)
	}
}
