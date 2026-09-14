package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"autoscale-distr-storage/internal/api"
	"autoscale-distr-storage/internal/document"
	"autoscale-distr-storage/internal/postgres"
	"autoscale-distr-storage/internal/telemetry"
	"autoscale-distr-storage/internal/topology"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func number(k string, d int) int {
	v, e := strconv.Atoi(env(k, strconv.Itoa(d)))
	if e != nil || v < 1 {
		panic("invalid " + k)
	}
	return v
}

func run() error {
	role, node := env("ROLE", "pu"), env("NODE_ID", "pu-1")
	if role != "pu" && role != "router" {
		return fmt.Errorf("ROLE must be pu or router")
	}
	a, err := topology.Parse(env("NODES", "pu-1=http://localhost:8081"), number("PARTITIONS", 128), env("ASSIGNMENT_EPOCH", "1"))
	if err != nil {
		return err
	}
	m := telemetry.New(role, node)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var executor document.Executor
	var ready func(context.Context) error
	var nodeID string
	client := &http.Client{Transport: &http.Transport{MaxIdleConns: 128, MaxIdleConnsPerHost: 64, MaxConnsPerHost: 128, IdleConnTimeout: 60 * time.Second, ResponseHeaderTimeout: 4 * time.Second}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if role == "pu" {
		found := false
		for _, n := range a.Nodes {
			if n.ID == node {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("NODE_ID not in assignment")
		}
		r, e := postgres.Open(ctx, os.Getenv("DATABASE_URL"), node, int32(number("POOL_MAX", 8)), m)
		if e != nil {
			return e
		}
		defer r.Pool.Close()
		executor = r
		nodeID = node
		// Readiness must verify schema presence as well as connectivity.
		ready = func(c context.Context) error {
			var exists bool
			if e := r.Pool.QueryRow(c, `SELECT to_regclass('public.documents') IS NOT NULL`).Scan(&exists); e != nil {
				return e
			}
			if !exists {
				return fmt.Errorf("schema missing")
			}
			return nil
		}
		for p := 0; p < a.Partitions; p++ {
			if a.Owner(p).ID == node {
				m.PartitionRequests.WithLabelValues(strconv.Itoa(p))
			}
		}
	} else {
		executor = &api.Router{Assignment: a, Client: client}
		ready = func(c context.Context) error {
			for _, n := range a.Nodes {
				req, _ := http.NewRequestWithContext(c, "GET", n.URL+"/readyz", nil)
				resp, e := client.Do(req)
				if e != nil {
					return e
				}
				resp.Body.Close()
				if resp.StatusCode != 200 {
					return fmt.Errorf("PU not ready")
				}
			}
			return nil
		}
	}
	m.Registry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "storage_topology_nodes", Help: "Configured node count."}, func() float64 { return float64(len(a.Nodes)) }))
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/livez", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if ctx.Err() != nil {
			api.Error(w, 503, "draining")
			return
		}
		c, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if ready(c) != nil {
			api.Error(w, 503, "not_ready")
			return
		}
		w.WriteHeader(200)
	})
	mux.Handle("/", &api.Handler{Executor: executor, Metrics: m, Assignment: a, NodeID: nodeID, Timeout: time.Duration(number("REQUEST_TIMEOUT_MS", 3000)) * time.Millisecond, MaxBody: 1 << 20, Slots: make(chan struct{}, number("MAX_INFLIGHT", 64))})
	srv := &http.Server{Addr: env("LISTEN_ADDR", ":8080"), Handler: mux, ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 6 * time.Second, IdleTimeout: 60 * time.Second}
	done := make(chan error, 1)
	go func() {
		slog.Info("listening", "role", role, "node", node, "address", srv.Addr)
		done <- srv.ListenAndServe()
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdown); err != nil {
		_ = srv.Close()
		return err
	}
	return nil
}

func main() {
	if err := run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("service failed", "error", err)
		os.Exit(1)
	}
}
