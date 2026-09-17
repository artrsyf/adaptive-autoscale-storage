package service

import (
	"context"
	"errors"
	"testing"

	document "autoscale-distr-storage/internal/document/domain"
)

type repositorySpy struct {
	calls    []string
	key      document.Key
	payload  document.Payload
	revision string
	ctx      context.Context
	err      error
}

func (r *repositorySpy) capture(ctx context.Context, op string, key document.Key, payload document.Payload, revision string) (document.Record, error) {
	r.calls = append(r.calls, op)
	r.ctx, r.key, r.payload, r.revision = ctx, key, payload, revision
	return document.Record{Key: key, Revision: "new-revision"}, r.err
}
func (r *repositorySpy) Create(ctx context.Context, k document.Key, p document.Payload) (document.Record, error) {
	return r.capture(ctx, "create", k, p, "")
}
func (r *repositorySpy) Get(ctx context.Context, k document.Key) (document.Record, error) {
	return r.capture(ctx, "get", k, nil, "")
}
func (r *repositorySpy) Replace(ctx context.Context, k document.Key, p document.Payload, v string) (document.Record, error) {
	return r.capture(ctx, "replace", k, p, v)
}
func (r *repositorySpy) Delete(ctx context.Context, k document.Key, v string) (document.Record, error) {
	return r.capture(ctx, "delete", k, nil, v)
}

func TestInvalidInputNeverReachesRepository(t *testing.T) {
	key := document.Key{PartitionKey: "tenant", ID: "doc"}
	for name, call := range map[string]func(*Service) error{
		"create key": func(s *Service) error {
			_, e := s.Create(context.Background(), document.Key{}, document.Payload(`{}`))
			return e
		},
		"create payload": func(s *Service) error { _, e := s.Create(context.Background(), key, document.Payload(`[]`)); return e },
		"get key":        func(s *Service) error { _, e := s.Get(context.Background(), document.Key{}); return e },
		"update revision": func(s *Service) error {
			_, e := s.Update(context.Background(), key, document.Payload(`{}`), "")
			return e
		},
		"update payload": func(s *Service) error {
			_, e := s.Update(context.Background(), key, document.Payload(`null`), "v1")
			return e
		},
		"delete revision": func(s *Service) error { _, e := s.Delete(context.Background(), key, ""); return e },
	} {
		t.Run(name, func(t *testing.T) {
			r := &repositorySpy{}
			if err := call(New(r)); !errors.Is(err, document.ErrInvalid) {
				t.Fatalf("error: %v", err)
			}
			if len(r.calls) != 0 {
				t.Fatalf("invalid input reached persistence: %v", r.calls)
			}
		})
	}
}

func TestUpdateDelegatesAtomicCheckWithoutPreReadOrRetry(t *testing.T) {
	key := document.Key{PartitionKey: "tenant", ID: "doc"}
	payload := document.Payload(`{"value":2}`)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, failure := range []error{nil, document.ErrConflict, document.ErrNotFound, context.Canceled} {
		r := &repositorySpy{err: failure}
		record, err := New(r).Update(ctx, key, payload, "v1")
		if !errors.Is(err, failure) {
			t.Fatalf("error lost: %v", err)
		}
		if len(r.calls) != 1 || r.calls[0] != "replace" || r.ctx != ctx || r.key != key || string(r.payload) != string(payload) || r.revision != "v1" {
			t.Fatalf("atomic contract changed: %+v", r)
		}
		if err == nil && record.Revision != "new-revision" {
			t.Fatal("result lost")
		}
	}
}
