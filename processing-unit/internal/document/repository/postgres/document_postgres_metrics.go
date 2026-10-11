package postgres

import "github.com/prometheus/client_golang/prometheus"

type DocumentPostgresMetrics struct {
	PoolWait   *prometheus.HistogramVec
	DBDuration *prometheus.HistogramVec
	Fenced     *prometheus.CounterVec
}

// NewDocumentPostgresMetrics регистрирует отдельные измерения ожидания пула и выполнения операций БД.
func NewDocumentPostgresMetrics(metricsRegistry *prometheus.Registry, node string) *DocumentPostgresMetrics {
	labels := prometheus.Labels{"role": "processing-unit", "node": node}
	buckets := []float64{.001, .0025, .005, .01, .025, .05, .1, .25, .5, 1, 2, 5}
	documentPostgresMetrics := &DocumentPostgresMetrics{
		PoolWait:   prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "storage_pool_acquire_seconds", Help: "Connection acquisition duration, including failed acquisitions.", ConstLabels: labels, Buckets: buckets}, []string{"operation", "outcome"}),
		DBDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "storage_db_operation_seconds", Help: "Database operation after acquisition, includes network, locks and commit; not server-only time.", ConstLabels: labels, Buckets: buckets}, []string{"operation", "outcome"}),
		Fenced:     prometheus.NewCounterVec(prometheus.CounterOpts{Name: "storage_fenced_requests_total", Help: "Commands rejected by authoritative assignment fencing.", ConstLabels: labels}, []string{"operation"}),
	}
	metricsRegistry.MustRegister(documentPostgresMetrics.PoolWait, documentPostgresMetrics.DBDuration, documentPostgresMetrics.Fenced)
	return documentPostgresMetrics
}
