package metrics

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

// This observer keeps Prometheus out of the processing service.
type PartitionRequestMetrics struct{ requests *prometheus.CounterVec }

// NewPartitionRequestMetrics регистрирует счётчик запросов по разделам для текущего исполнителя.
func NewPartitionRequestMetrics(registry *prometheus.Registry, nodeID string) *PartitionRequestMetrics {
	partitionRequestMetrics := &PartitionRequestMetrics{requests: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "storage_partition_requests_total", Help: "Requests reaching an assigned processing unit, including rejected overload.", ConstLabels: prometheus.Labels{"role": "processing-unit", "node": nodeID}}, []string{"partition"})}
	registry.MustRegister(partitionRequestMetrics.requests)
	return partitionRequestMetrics
}

// RecordPartitionRequest учитывает запрос к разделу до проверки перегрузки исполнителя.
func (partitionRequestMetrics *PartitionRequestMetrics) RecordPartitionRequest(partitionID int) {
	partitionRequestMetrics.requests.WithLabelValues(strconv.Itoa(partitionID)).Inc()
}
