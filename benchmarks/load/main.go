// load is a bounded open-loop HTTP benchmark with Prometheus and raw results.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type config struct {
	Target, Scenario, Distribution                    string
	Rate, ReadPercent, Records, PayloadBytes, Workers int
	Seed                                              int64
	Duration, Warmup                                  time.Duration
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func integer(k string, d int) int {
	v, e := strconv.Atoi(env(k, strconv.Itoa(d)))
	if e != nil {
		panic(k)
	}
	return v
}
func duration(k, d string) time.Duration {
	v, e := time.ParseDuration(env(k, d))
	if e != nil {
		panic(k)
	}
	return v
}

type result struct {
	At        time.Time `json:"at"`
	Phase     string    `json:"phase"`
	Operation string    `json:"operation"`
	Status    int       `json:"status"`
	Seconds   float64   `json:"seconds"`
}
type summary struct {
	Started             time.Time
	Finished            time.Time
	MeasurementStarted  time.Time
	MeasurementFinished time.Time
	Completed           int
	Success             int
	Conflicts           int
	Errors              int
	Dropped             int64
	P50Seconds          float64
	P95Seconds          float64
	P99Seconds          float64
}

func run() error {
	c := config{env("TARGET", "http://localhost:8080"), env("SCENARIO", "constant"), env("DISTRIBUTION", "uniform"), integer("RATE", 100), integer("READ_PERCENT", 80), integer("RECORDS", 4096), integer("PAYLOAD_BYTES", 1024), integer("WORKERS", 128), int64(integer("SEED", 42)), duration("DURATION", "60s"), duration("WARMUP", "10s")}
	if c.Rate < 1 || c.Rate > 100000 || c.ReadPercent < 0 || c.ReadPercent > 100 || c.Records < 128 || c.PayloadBytes < 16 || c.PayloadBytes > 900000 || c.Workers < 1 || c.Workers > 4096 || c.Duration <= 0 || c.Warmup < 0 {
		return fmt.Errorf("invalid workload settings")
	}
	if c.Scenario != "constant" && c.Scenario != "ramp" && c.Scenario != "burst" {
		return fmt.Errorf("unknown scenario")
	}
	if c.Distribution != "uniform" && c.Distribution != "hot" {
		return fmt.Errorf("unknown distribution")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	start := time.Now().UTC()
	dir := filepath.Join(env("OUTPUT", "results"), start.Format("20060102T150405.000000000Z"))
	if e := os.MkdirAll(dir, 0755); e != nil {
		return e
	}
	manifest, _ := json.MarshalIndent(c, "", "  ")
	if e := os.WriteFile(filepath.Join(dir, "config.json"), manifest, 0644); e != nil {
		return e
	}
	f, e := os.Create(filepath.Join(dir, "requests.jsonl"))
	if e != nil {
		return e
	}
	defer f.Close()
	registry := prometheus.NewRegistry()
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "bench_requests_total", Help: "Completed HTTP requests, including update pre-reads."}, []string{"phase", "operation", "status"})
	latency := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "bench_request_duration_seconds", Help: "Client HTTP duration.", Buckets: []float64{.001, .0025, .005, .01, .025, .05, .1, .25, .5, 1, 2, 5}}, []string{"phase", "operation"})
	offered := prometheus.NewGauge(prometheus.GaugeOpts{Name: "bench_offered_iterations_per_second", Help: "Target logical iterations per second. Updates include a pre-read."})
	drops := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "bench_dropped_iterations_total", Help: "Scheduled iterations not started; generator concurrency or scheduler exhausted."}, []string{"phase"})
	active := prometheus.NewGauge(prometheus.GaugeOpts{Name: "bench_active_workers", Help: "Active workload iterations."})
	phase := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "bench_phase", Help: "Current phase, one hot."}, []string{"phase"})
	registry.MustRegister(requests, latency, offered, drops, active, phase, prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	for _, p := range []string{"seed", "warmup", "measure", "finished"} {
		phase.WithLabelValues(p).Set(0)
		drops.WithLabelValues(p)
	}
	setPhase := func(p string) {
		for _, s := range []string{"seed", "warmup", "measure", "finished"} {
			v := 0.
			if s == p {
				v = 1
			}
			phase.WithLabelValues(s).Set(v)
		}
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "benchmark metrics available") })
	srv := &http.Server{Addr: ":9091", Handler: mux, ReadHeaderTimeout: time.Second}
	go func() {
		if e := srv.ListenAndServe(); e != nil && e != http.ErrServerClosed {
			fmt.Fprintln(os.Stderr, e)
			stop()
		}
	}()
	defer srv.Close()
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{MaxIdleConns: c.Workers * 2, MaxIdleConnsPerHost: c.Workers, MaxConnsPerHost: c.Workers}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	var mu sync.Mutex
	var samples []float64
	s := summary{Started: start}
	buffer := bufio.NewWriterSize(f, 256*1024)
	defer buffer.Flush()
	encoder := json.NewEncoder(buffer)
	var writeErr error
	call := func(p, op string, command commandDTO) (recordDTO, int) {
		var record recordDTO
		data, _ := json.Marshal(command)
		req, _ := http.NewRequestWithContext(ctx, "POST", c.Target+"/"+op, strings.NewReader(string(data)))
		req.Header.Set("Content-Type", "application/json")
		t := time.Now()
		resp, err := client.Do(req)
		status := 0
		if err == nil {
			status = resp.StatusCode
			if status < 300 {
				if json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&record) != nil {
					status = 0
				}
			} else {
				_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			}
			resp.Body.Close()
		}
		seconds := time.Since(t).Seconds()
		requests.WithLabelValues(p, op, strconv.Itoa(status)).Inc()
		latency.WithLabelValues(p, op).Observe(seconds)
		mu.Lock()
		if e := encoder.Encode(result{time.Now().UTC(), p, op, status, seconds}); e != nil && writeErr == nil {
			writeErr = e
			stop()
		}
		if p == "measure" {
			s.Completed++
			if status >= 200 && status < 300 {
				s.Success++
			} else if status == 409 {
				s.Conflicts++
			} else {
				s.Errors++
			}
			samples = append(samples, seconds)
		}
		mu.Unlock()
		return record, status
	}
	keys := make([]string, c.Records)
	var hot []int
	for i := range keys {
		keys[i] = fmt.Sprintf("bench-%d", i)
		if partition(keys[i]) == 0 {
			hot = append(hot, i)
		}
	}
	if len(hot) == 0 {
		return fmt.Errorf("dataset has no partition zero keys")
	}
	payload, _ := json.Marshal(map[string]string{"value": strings.Repeat("x", c.PayloadBytes-12)})
	setPhase("seed")
	// Sequential seed is outside measured phases and fails on unexpected errors.
	for i, key := range keys {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		command := commandDTO{PartitionKey: key, ID: "record", Payload: payload}
		_, code := call("seed", "create", command)
		if code != 201 && code != 409 {
			return fmt.Errorf("seed %d status %d", i, code)
		}
		if code == 409 {
			record, status := call("seed", "get", commandDTO{PartitionKey: key, ID: "record"})
			if status != 200 {
				return fmt.Errorf("seed read %d status %d", i, status)
			}
			command.ExpectedRevision = record.Revision
			if _, status = call("seed", "update", command); status != 200 {
				return fmt.Errorf("seed reset %d status %d", i, status)
			}
		}
	}
	rng := rand.New(rand.NewSource(c.Seed))
	var wg sync.WaitGroup
	slots := make(chan struct{}, c.Workers)
	var dropped atomic.Int64
	for _, p := range []struct {
		name     string
		duration time.Duration
	}{{"warmup", c.Warmup}, {"measure", c.Duration}} {
		setPhase(p.name)
		begin := time.Now()
		if p.name == "measure" {
			s.MeasurementStarted = begin.UTC()
		}
		next := begin
		end := begin.Add(p.duration)
		for next.Before(end) && ctx.Err() == nil {
			rate := float64(c.Rate)
			fraction := float64(next.Sub(begin)) / float64(p.duration)
			if p.name == "measure" {
				switch c.Scenario {
				case "ramp":
					rate *= .2 + .8*fraction
				case "burst":
					if fraction >= .4 && fraction < .6 {
						rate *= 3
					}
				}
			}
			offered.Set(rate)
			interval := time.Duration(float64(time.Second) / rate)
			if delay := time.Until(next); delay > 0 {
				timer := time.NewTimer(delay)
				select {
				case <-timer.C:
				case <-ctx.Done():
					timer.Stop()
				}
			}
			// Skip overdue slots instead of issuing a catch-up burst.
			late := time.Since(next)
			if late > interval*2 {
				skip := int64(late / interval)
				remaining := int64(end.Sub(next) / interval)
				if skip > remaining {
					skip = remaining
				}
				drops.WithLabelValues(p.name).Add(float64(skip))
				if p.name == "measure" {
					dropped.Add(skip)
				}
				next = next.Add(time.Duration(skip) * interval)
				if !next.Before(end) {
					break
				}
			}
			i := rng.Intn(c.Records)
			if c.Distribution == "hot" && rng.Intn(100) < 80 {
				i = hot[rng.Intn(len(hot))]
			}
			read := rng.Intn(100) < c.ReadPercent
			select {
			case slots <- struct{}{}:
				wg.Add(1)
				active.Inc()
				go func(p string, i int, read bool) {
					defer wg.Done()
					defer func() { <-slots; active.Dec() }()
					command := commandDTO{PartitionKey: keys[i], ID: "record"}
					record, code := call(p, "get", command)
					if !read && code == 200 {
						command.Payload = payload
						command.ExpectedRevision = record.Revision
						call(p, "update", command)
					}
				}(p.name, i, read)
			default:
				drops.WithLabelValues(p.name).Inc()
				if p.name == "measure" {
					dropped.Add(1)
				}
			}
			next = next.Add(interval)
		}
		wg.Wait()
		if p.name == "measure" {
			s.MeasurementFinished = time.Now().UTC()
		}
	}
	offered.Set(0)
	setPhase("finished")
	s.Finished = time.Now().UTC()
	s.Dropped = dropped.Load()
	sort.Float64s(samples)
	quantile := func(q float64) float64 {
		if len(samples) == 0 {
			return 0
		}
		return samples[int(q*float64(len(samples)-1))]
	}
	s.P50Seconds = quantile(.5)
	s.P95Seconds = quantile(.95)
	s.P99Seconds = quantile(.99)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if writeErr != nil {
		return writeErr
	}
	if err := buffer.Flush(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(s, "", "  ")
	if e := os.WriteFile(filepath.Join(dir, "summary.json"), data, 0644); e != nil {
		return e
	}
	metrics, err := snapshotMetrics(registry)
	if err != nil {
		return err
	}
	if e := os.WriteFile(filepath.Join(dir, "metrics.prom"), metrics, 0644); e != nil {
		return e
	}
	fmt.Printf("COMPLETE %s\n%s\n", dir, data)
	if env("HOLD", "true") == "true" {
		<-ctx.Done()
	}
	return nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
