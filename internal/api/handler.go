package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"autoscale-distr-storage/internal/document"
	"autoscale-distr-storage/internal/telemetry"
	"autoscale-distr-storage/internal/topology"
)

type Handler struct {
	Executor   document.Executor
	Metrics    *telemetry.Metrics
	Assignment *topology.Assignment
	NodeID     string // Empty for router.
	Timeout    time.Duration
	MaxBody    int64
	Slots      chan struct{}
}

func Error(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	status := http.StatusOK
	op := strings.TrimPrefix(r.URL.Path, "/")
	if !document.ValidOperation(op) {
		op = "unknown"
	}
	defer func() {
		h.Metrics.Requests.WithLabelValues(op, strconv.Itoa(status)).Inc()
		h.Metrics.Latency.WithLabelValues(op).Observe(time.Since(start).Seconds())
	}()
	fail := func(s int, code string) { status = s; Error(w, s, code) }
	if op == "unknown" {
		fail(404, "unknown_operation")
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		fail(405, "method_not_allowed")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, h.MaxBody)
	var c document.Command
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		var size *http.MaxBytesError
		if errors.As(err, &size) {
			fail(413, "body_too_large")
		} else {
			fail(400, "invalid_json")
		}
		return
	}
	if err := d.Decode(new(any)); err != io.EOF {
		var size *http.MaxBytesError
		if errors.As(err, &size) {
			fail(413, "body_too_large")
		} else {
			fail(400, "invalid_json")
		}
		return
	}
	if c.Validate(op) != nil {
		fail(400, "invalid_request")
		return
	}
	partition := h.Assignment.Partition(c.PartitionKey)
	if h.NodeID != "" {
		if r.Header.Get("X-Assignment-Epoch") != h.Assignment.Epoch || h.Assignment.Owner(partition).ID != h.NodeID {
			fail(409, "wrong_assignment")
			return
		}
		h.Metrics.PartitionRequests.WithLabelValues(strconv.Itoa(partition)).Inc()
	}
	select {
	case h.Slots <- struct{}{}:
		defer func() { <-h.Slots }()
	default:
		fail(429, "overloaded")
		return
	}
	h.Metrics.Active.Inc()
	defer h.Metrics.Active.Dec()
	ctx, cancel := context.WithTimeout(r.Context(), h.Timeout)
	defer cancel()
	record, err := h.Executor.Execute(ctx, op, c)
	if err != nil {
		switch {
		case errors.Is(err, document.ErrInvalid):
			fail(400, "invalid_request")
		case errors.Is(err, document.ErrNotFound):
			fail(404, "not_found")
		case errors.Is(err, document.ErrConflict):
			fail(409, "revision_conflict")
		case errors.Is(err, document.ErrExists):
			fail(409, "already_exists")
		case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
			fail(504, "deadline_exceeded")
		default:
			var upstream *UpstreamError
			if errors.As(err, &upstream) {
				fail(upstream.Status, upstream.Code)
			} else {
				fail(503, "database_unavailable")
			}
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Partition-ID", strconv.Itoa(partition))
	w.Header().Set("X-PU-ID", h.Assignment.Owner(partition).ID)
	if op == "create" {
		status = 201
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(record)
}
