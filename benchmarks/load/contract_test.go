package main

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestPartitionContract(t *testing.T) {
	if got := partition("abc"); got != 106 {
		t.Fatalf("partition=%d, want 106", got)
	}
}

func TestMetricsSnapshot(t *testing.T) {
	r := prometheus.NewRegistry()
	c := prometheus.NewCounter(prometheus.CounterOpts{Name: "bench_snapshot_test_total", Help: "Snapshot test."})
	r.MustRegister(c)
	c.Add(2)
	data, err := snapshotMetrics(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "bench_snapshot_test_total 2") {
		t.Fatalf("missing metric: %s", data)
	}
}
