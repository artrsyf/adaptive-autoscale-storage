package api

import "github.com/prometheus/client_golang/prometheus"

type DocumentHttpMetrics struct {
	Requests *prometheus.CounterVec
	Latency  *prometheus.HistogramVec
	Active   prometheus.Gauge
}

// NewDocumentHttpMetrics регистрирует счётчики запросов, задержек и активных вызовов для роли и ноды.
func NewDocumentHttpMetrics(metricsRegistry *prometheus.Registry, role, node string) *DocumentHttpMetrics {
	labels := prometheus.Labels{"role": role, "node": node}
	buckets := []float64{.001, .0025, .005, .01, .025, .05, .1, .25, .5, 1, 2, 5}
	documentHttpMetrics := &DocumentHttpMetrics{
		Requests: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "storage_requests_total", Help: "Completed API requests.", ConstLabels: labels}, []string{"operation", "status"}),
		Latency:  prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "storage_request_duration_seconds", Help: "API request duration.", ConstLabels: labels, Buckets: buckets}, []string{"operation"}),
		Active:   prometheus.NewGauge(prometheus.GaugeOpts{Name: "storage_active_requests", Help: "In-flight command service calls.", ConstLabels: labels}),
	}
	metricsRegistry.MustRegister(documentHttpMetrics.Requests, documentHttpMetrics.Latency, documentHttpMetrics.Active)
	for _, operation := range []string{"create", "get", "update", "delete", "unknown"} {
		for _, status := range []string{"200", "201", "400", "404", "405", "409", "413", "429", "500", "502", "503", "504"} {
			documentHttpMetrics.Requests.WithLabelValues(operation, status)
		}
	}
	return documentHttpMetrics
}
