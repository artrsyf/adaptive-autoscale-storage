package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"autoscale-distr-storage/internal/document/domain"
	"autoscale-distr-storage/internal/platform/telemetry"
)

func TestIntegrationDocumentLifecycle(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	r, e := Open(ctx, dsn, "integration", 4, telemetry.New("pu", "integration"))
	if e != nil {
		t.Fatal(e)
	}
	defer r.pool.Close()
	key := document.Key{PartitionKey: fmt.Sprintf("test-%d", time.Now().UnixNano()), ID: "doc"}
	payload := document.Payload(`{"value":1}`)
	var expectedRevision string
	defer r.pool.Exec(context.Background(), `DELETE FROM documents WHERE partition_key=$1`, key.PartitionKey)
	first, e := r.Create(ctx, key, payload)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = r.Create(ctx, key, payload); !errors.Is(e, document.ErrExists) {
		t.Fatalf("duplicate: %v", e)
	}
	expectedRevision = first.Revision
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := r.Replace(ctx, key, payload, expectedRevision); results <- e }()
	}
	wg.Wait()
	close(results)
	wins, conflicts := 0, 0
	for e := range results {
		if e == nil {
			wins++
		} else if errors.Is(e, document.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	if wins != 1 || conflicts != 7 {
		t.Fatalf("wins=%d conflicts=%d", wins, conflicts)
	}

	current, e := r.Get(ctx, key)
	if e != nil {
		t.Fatal(e)
	}
	deleteRevision := current.Revision
	if _, e = r.Delete(ctx, key, deleteRevision); e != nil {
		t.Fatal(e)
	}
	expectedRevision = ""
	recreated, e := r.Create(ctx, key, payload)
	if e != nil {
		t.Fatal(e)
	}
	if recreated.Revision == first.Revision || recreated.Revision == current.Revision {
		t.Fatal("revision reused")
	}
	if _, e = r.Delete(ctx, key, deleteRevision); !errors.Is(e, document.ErrConflict) {
		t.Fatalf("stale delete: %v", e)
	}
	cancelled, cancelNow := context.WithCancel(ctx)
	cancelNow()
	if _, e = r.Get(cancelled, key); !errors.Is(e, context.Canceled) {
		t.Fatalf("cancellation: %v", e)
	}
	// A real row lock must time out without changing the record.
	lock, e := r.pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = lock.Exec(ctx, `SELECT 1 FROM documents WHERE partition_key=$1 AND id=$2 FOR UPDATE`, key.PartitionKey, key.ID); e != nil {
		t.Fatal(e)
	}
	short, stop := context.WithTimeout(ctx, 100*time.Millisecond)
	expectedRevision = recreated.Revision
	_, e = r.Replace(short, key, payload, expectedRevision)
	stop()
	_ = lock.Rollback(ctx)
	if !errors.Is(e, context.DeadlineExceeded) {
		t.Fatalf("locked update: %v", e)
	}
	after, e := r.Get(ctx, key)
	if e != nil || after.Revision != recreated.Revision {
		t.Fatalf("timeout changed record: %v", e)
	}
}
