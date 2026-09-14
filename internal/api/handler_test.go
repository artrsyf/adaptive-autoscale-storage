package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"autoscale-distr-storage/internal/document"
	"autoscale-distr-storage/internal/telemetry"
	"autoscale-distr-storage/internal/topology"
)

type executorFunc func(context.Context, string, document.Command) (document.Record, error)

func (f executorFunc) Execute(c context.Context, o string, d document.Command) (document.Record, error) {
	return f(c, o, d)
}
func TestValidationAndErrors(t *testing.T) {
	a, _ := topology.Parse("pu-1=http://pu-1:8080", 128, "1")
	for _, tc := range []struct {
		name, body string
		err        error
		status     int
	}{
		{"valid", `{"partition_key":"a","id":"b"}`, nil, 200},
		{"unknown_field", `{"partition_key":"a","id":"b","sql":"drop"}`, nil, 400},
		{"multiple_json", `{"partition_key":"a","id":"b"} {}`, nil, 400},
		{"missing_key", `{"id":"b"}`, nil, 400},
		{"not_found", `{"partition_key":"a","id":"b"}`, document.ErrNotFound, 404},
		{"unavailable", `{"partition_key":"a","id":"b"}`, errors.New("private database URL"), 503},
		{"timeout", `{"partition_key":"a","id":"b"}`, context.DeadlineExceeded, 504},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &Handler{Assignment: a, Metrics: telemetry.New("router", "router"), MaxBody: 1024, Timeout: time.Second, Slots: make(chan struct{}, 1), Executor: executorFunc(func(context.Context, string, document.Command) (document.Record, error) {
				return document.Record{}, tc.err
			})}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("POST", "/get", strings.NewReader(tc.body)))
			if w.Code != tc.status {
				t.Fatalf("got %d: %s", w.Code, w.Body)
			}
			if strings.Contains(w.Body.String(), "private") {
				t.Fatal("internal error exposed")
			}
		})
	}
}
func TestOwnershipAndOverload(t *testing.T) {
	a, _ := topology.Parse("pu-1=http://pu-1:8080", 128, "1")
	h := &Handler{Assignment: a, NodeID: "pu-1", Metrics: telemetry.New("pu", "pu-1"), MaxBody: 1024, Timeout: time.Second, Slots: make(chan struct{}, 1)}
	for _, tc := range []struct {
		epoch  string
		status int
	}{{"", 409}, {"0", 409}, {"1", 429}} {
		r := httptest.NewRequest("POST", "/get", strings.NewReader(`{"partition_key":"a","id":"b"}`))
		r.Header.Set("X-Assignment-Epoch", tc.epoch)
		h.Slots <- struct{}{}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		<-h.Slots
		if w.Code != tc.status {
			t.Fatalf("got %d", w.Code)
		}
	}
}
func TestRouterForwardsOnce(t *testing.T) {
	count := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if r.Header.Get("X-Assignment-Epoch") != "7" {
			t.Error("epoch missing")
		}
		Error(w, 409, "revision_conflict")
	}))
	defer s.Close()
	a, _ := topology.Parse("pu-1="+s.URL, 128, "7")
	r := &Router{a, s.Client()}
	_, err := r.Execute(context.Background(), "update", document.Command{PartitionKey: "a", ID: "b"})
	var u *UpstreamError
	if !errors.As(err, &u) || u.Status != 409 || count != 1 {
		t.Fatalf("count %d err %v", count, err)
	}
}
