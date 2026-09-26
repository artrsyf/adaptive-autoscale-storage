package main

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// TestPartitionContract фиксирует совместимость хеширования нагрузочного клиента с Router.
func TestPartitionContract(test *testing.T) {
	if actualPartition := partition("abc", 128); actualPartition != 106 {
		test.Fatalf("partition=%d, want 106", actualPartition)
	}
}

// TestMetricsSnapshot проверяет, что снимок Prometheus содержит зарегистрированное значение счётчика.
func TestMetricsSnapshot(test *testing.T) {
	metricsRegistry := prometheus.NewRegistry()
	counter := prometheus.NewCounter(prometheus.CounterOpts{Name: "bench_snapshot_test_total", Help: "Snapshot test."})
	metricsRegistry.MustRegister(counter)
	counter.Add(2)
	data, err := snapshotMetrics(metricsRegistry)
	if err != nil {
		test.Fatal(err)
	}
	if !strings.Contains(string(data), "bench_snapshot_test_total 2") {
		test.Fatalf("missing metric: %s", data)
	}
}
