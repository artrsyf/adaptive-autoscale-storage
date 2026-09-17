package telemetry

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	Registry          *prometheus.Registry
	Requests          *prometheus.CounterVec
	Latency           *prometheus.HistogramVec
	Active            prometheus.Gauge
	PoolWait          *prometheus.HistogramVec
	DBDuration        *prometheus.HistogramVec
	PartitionRequests *prometheus.CounterVec
}

func New(role, node string) *Metrics {
	r := prometheus.NewRegistry()
	labels := prometheus.Labels{"role": role, "node": node}
	buckets := []float64{.001, .0025, .005, .01, .025, .05, .1, .25, .5, 1, 2, 5}
	m := &Metrics{Registry: r,
		Requests:          prometheus.NewCounterVec(prometheus.CounterOpts{Name: "storage_requests_total", Help: "Completed API requests.", ConstLabels: labels}, []string{"operation", "status"}),
		Latency:           prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "storage_request_duration_seconds", Help: "API request duration.", ConstLabels: labels, Buckets: buckets}, []string{"operation"}),
		Active:            prometheus.NewGauge(prometheus.GaugeOpts{Name: "storage_active_requests", Help: "Admitted in-flight API requests.", ConstLabels: labels}),
		PoolWait:          prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "storage_pool_acquire_seconds", Help: "Connection acquisition duration, including failed acquisitions.", ConstLabels: labels, Buckets: buckets}, []string{"operation", "outcome"}),
		DBDuration:        prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "storage_db_operation_seconds", Help: "Database operation after acquisition, includes network, locks and commit; not server-only time.", ConstLabels: labels, Buckets: buckets}, []string{"operation", "outcome"}),
		PartitionRequests: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "storage_partition_requests_total", Help: "Requests reaching an assigned PU, including rejected overload.", ConstLabels: labels}, []string{"partition"}),
	}
	r.MustRegister(m.Requests, m.Latency, m.Active, m.PoolWait, m.DBDuration, m.PartitionRequests, prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	for _, op := range []string{"create", "get", "update", "delete", "unknown"} {
		for _, status := range []string{"200", "201", "400", "404", "405", "409", "413", "429", "500", "502", "503", "504"} {
			m.Requests.WithLabelValues(op, status)
		}
	}
	return m
}
