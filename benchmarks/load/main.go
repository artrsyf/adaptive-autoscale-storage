// load is a bounded open-loop HTTP benchmark with Prometheus and raw results.
package main

import (
	"autoscale-distr-storage-benchmarks/transport/dto"
	"bufio"
	"context"
	"encoding/json"
	"flag"
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

// run загружает параметры, заполняет данные, выполняет прогрев и измерение, затем сохраняет результаты нагрузки.
func run() error {
	path := flag.String("config", "config.yaml", "workload YAML configuration")
	scenario := flag.String("scenario", "", "override scenario")
	distribution := flag.String("distribution", "", "override distribution")
	rate := flag.Int("rate", 0, "override iterations per second")
	seconds := flag.Int("seconds", 0, "override measurement duration")
	reads := flag.Int("read-percent", -1, "override read percentage")
	warmup := flag.Duration("warmup", -1, "override warm-up duration")
	records := flag.Int("records", 0, "override record count")
	flag.Parse()
	workloadConfig, err := loadConfig(*path)
	if err != nil {
		return err
	}
	if *scenario != "" {
		workloadConfig.Scenario = *scenario
	}
	if *distribution != "" {
		workloadConfig.Distribution = *distribution
	}
	if *rate != 0 {
		workloadConfig.Rate = *rate
	}
	if *seconds != 0 {
		workloadConfig.Duration = duration(time.Duration(*seconds) * time.Second)
	}
	if *reads != -1 {
		workloadConfig.ReadPercent = *reads
	}
	if *warmup >= 0 {
		workloadConfig.Warmup = duration(*warmup)
	}
	if *records != 0 {
		workloadConfig.Records = *records
	}

	if workloadConfig.Rate < 1 || workloadConfig.Rate > 100000 || workloadConfig.ReadPercent < 0 || workloadConfig.ReadPercent > 100 || workloadConfig.Records < 128 || workloadConfig.PayloadBytes < 16 || workloadConfig.PayloadBytes > 900000 || workloadConfig.Workers < 1 || workloadConfig.Workers > 4096 || workloadConfig.Duration <= 0 || workloadConfig.Warmup < 0 {
		return fmt.Errorf("invalid workload settings")
	}
	if workloadConfig.Scenario != "constant" && workloadConfig.Scenario != "ramp" && workloadConfig.Scenario != "burst" {
		return fmt.Errorf("unknown scenario")
	}
	if workloadConfig.Distribution != "uniform" && workloadConfig.Distribution != "hot" {
		return fmt.Errorf("unknown distribution")
	}
	operationContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	runStartedAt := time.Now().UTC()
	outputDirectory := filepath.Join(workloadConfig.Output, runStartedAt.Format("20060102T150405.000000000Z"))
	if operationError := os.MkdirAll(outputDirectory, 0755); operationError != nil {
		return operationError
	}
	manifest, _ := json.MarshalIndent(workloadConfig, "", "  ")
	if operationError := os.WriteFile(filepath.Join(outputDirectory, "config.json"), manifest, 0644); operationError != nil {
		return operationError
	}
	requestsFile, operationError := os.Create(filepath.Join(outputDirectory, "requests.jsonl"))
	if operationError != nil {
		return operationError
	}
	defer requestsFile.Close()
	registry := prometheus.NewRegistry()
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "bench_requests_total", Help: "Completed HTTP requests, including update pre-reads."}, []string{"phase", "operation", "status"})
	latency := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "bench_request_duration_seconds", Help: "Client HTTP duration.", Buckets: []float64{.001, .0025, .005, .01, .025, .05, .1, .25, .5, 1, 2, 5}}, []string{"phase", "operation"})
	offered := prometheus.NewGauge(prometheus.GaugeOpts{Name: "bench_offered_iterations_per_second", Help: "Target logical iterations per second. Updates include a pre-read."})
	drops := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "bench_dropped_iterations_total", Help: "Scheduled iterations not started; generator concurrency or scheduler exhausted."}, []string{"phase"})
	active := prometheus.NewGauge(prometheus.GaugeOpts{Name: "bench_active_workers", Help: "Active workload iterations."})
	phase := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "bench_phase", Help: "Current phase, one hot."}, []string{"phase"})
	registry.MustRegister(requests, latency, offered, drops, active, phase, prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	for _, phaseName := range []string{"seed", "warmup", "measure", "finished"} {
		phase.WithLabelValues(phaseName).Set(0)
		drops.WithLabelValues(phaseName)
	}
	setPhase := func(phaseName string) {
		for _, phaseLabel := range []string{"seed", "warmup", "measure", "finished"} {
			phaseActive := 0.
			if phaseLabel == phaseName {
				phaseActive = 1
			}
			phase.WithLabelValues(phaseLabel).Set(phaseActive)
		}
	}
	httpMux := http.NewServeMux()
	httpMux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	httpMux.HandleFunc("/status", func(responseWriter http.ResponseWriter, httpRequest *http.Request) {
		fmt.Fprint(responseWriter, "benchmark metrics available")
	})
	httpServer := &http.Server{Addr: ":" + strconv.Itoa(workloadConfig.MetricsPort), Handler: httpMux, ReadHeaderTimeout: time.Second}
	go func() {
		if operationError := httpServer.ListenAndServe(); operationError != nil && operationError != http.ErrServerClosed {
			fmt.Fprintln(os.Stderr, operationError)
			stop()
		}
	}()
	defer httpServer.Close()
	httpClient := &http.Client{Timeout: time.Duration(workloadConfig.RequestTimeout), Transport: &http.Transport{MaxIdleConns: workloadConfig.Workers * 2, MaxIdleConnsPerHost: workloadConfig.Workers, MaxConnsPerHost: workloadConfig.Workers}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	var resultsMutex sync.Mutex
	var latencySamples []float64
	runSummary := summary{Started: runStartedAt}
	buffer := bufio.NewWriterSize(requestsFile, 256*1024)
	defer buffer.Flush()
	encoder := json.NewEncoder(buffer)
	var writeErr error
	sendDocumentCommand := func(phaseName, operation string, command any) (string, int) {
		var documentRevision string
		data, _ := json.Marshal(command)
		httpRequest, _ := http.NewRequestWithContext(operationContext, "POST", workloadConfig.Target+"/"+operation, strings.NewReader(string(data)))
		httpRequest.Header.Set("Content-Type", "application/json")
		requestStartedAt := time.Now()
		httpResponse, err := httpClient.Do(httpRequest)
		status := 0
		if err == nil {
			status = httpResponse.StatusCode
			if status < 300 {
				var decodeErr error
				documentRevision, decodeErr = decodeDocumentRevision(json.NewDecoder(io.LimitReader(httpResponse.Body, 2<<20)), operation)
				if decodeErr != nil {
					status = 0
				}
			} else {
				_, _ = io.Copy(io.Discard, io.LimitReader(httpResponse.Body, 4096))
			}
			httpResponse.Body.Close()
		}
		seconds := time.Since(requestStartedAt).Seconds()
		requests.WithLabelValues(phaseName, operation, strconv.Itoa(status)).Inc()
		latency.WithLabelValues(phaseName, operation).Observe(seconds)
		resultsMutex.Lock()
		if operationError := encoder.Encode(result{time.Now().UTC(), phaseName, operation, status, seconds}); operationError != nil && writeErr == nil {
			writeErr = operationError
			stop()
		}
		if phaseName == "measure" {
			runSummary.Completed++
			if status >= 200 && status < 300 {
				runSummary.Success++
			} else if status == 409 {
				runSummary.Conflicts++
			} else {
				runSummary.Errors++
			}
			latencySamples = append(latencySamples, seconds)
		}
		resultsMutex.Unlock()
		return documentRevision, status
	}
	keys := make([]string, workloadConfig.Records)
	var hotRecordIndexes []int
	for recordIndex := range keys {
		keys[recordIndex] = fmt.Sprintf("bench-%d", recordIndex)
		if partition(keys[recordIndex], workloadConfig.Partitions) == 0 {
			hotRecordIndexes = append(hotRecordIndexes, recordIndex)
		}
	}
	if len(hotRecordIndexes) == 0 {
		return fmt.Errorf("dataset has no partition zero keys")
	}
	payload, _ := json.Marshal(map[string]string{"value": strings.Repeat("x", workloadConfig.PayloadBytes-12)})
	setPhase("seed")
	// Sequential seed is outside measured phases and fails on unexpected errors.
	for recordIndex, key := range keys {
		if operationContext.Err() != nil {
			return operationContext.Err()
		}
		command := dto.DocumentCreateRequest{PartitionKey: key, ID: "record", Payload: payload}
		_, code := sendDocumentCommand("seed", "create", command)
		if code != 201 && code != 409 {
			return fmt.Errorf("seed %d status %d", recordIndex, code)
		}
		if code == 409 {
			documentRevision, status := sendDocumentCommand("seed", "get", dto.DocumentGetRequest{PartitionKey: key, ID: "record"})
			if status != 200 {
				return fmt.Errorf("seed read %d status %d", recordIndex, status)
			}
			update := dto.DocumentUpdateRequest{PartitionKey: key, ID: "record", Payload: payload, ExpectedRevision: documentRevision}
			if _, status = sendDocumentCommand("seed", "update", update); status != 200 {
				return fmt.Errorf("seed reset %d status %d", recordIndex, status)
			}
		}
	}
	randomGenerator := rand.New(rand.NewSource(workloadConfig.Seed))
	var workloadWorkers sync.WaitGroup
	workerSlots := make(chan struct{}, workloadConfig.Workers)
	var dropped atomic.Int64
	for _, phase := range []struct {
		name     string
		duration time.Duration
	}{{"warmup", time.Duration(workloadConfig.Warmup)}, {"measure", time.Duration(workloadConfig.Duration)}} {
		setPhase(phase.name)
		phaseStartedAt := time.Now()
		if phase.name == "measure" {
			runSummary.MeasurementStarted = phaseStartedAt.UTC()
		}
		nextIterationAt := phaseStartedAt
		phaseEndsAt := phaseStartedAt.Add(phase.duration)
		for nextIterationAt.Before(phaseEndsAt) && operationContext.Err() == nil {
			rate := float64(workloadConfig.Rate)
			fraction := float64(nextIterationAt.Sub(phaseStartedAt)) / float64(phase.duration)
			if phase.name == "measure" {
				switch workloadConfig.Scenario {
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
			if delay := time.Until(nextIterationAt); delay > 0 {
				timer := time.NewTimer(delay)
				select {
				case <-timer.C:
				case <-operationContext.Done():
					timer.Stop()
				}
			}
			// Skip overdue slots instead of issuing a catch-up burst.
			late := time.Since(nextIterationAt)
			if late > interval*2 {
				skippedIterations := int64(late / interval)
				remainingIterations := int64(phaseEndsAt.Sub(nextIterationAt) / interval)
				if skippedIterations > remainingIterations {
					skippedIterations = remainingIterations
				}
				drops.WithLabelValues(phase.name).Add(float64(skippedIterations))
				if phase.name == "measure" {
					dropped.Add(skippedIterations)
				}
				nextIterationAt = nextIterationAt.Add(time.Duration(skippedIterations) * interval)
				if !nextIterationAt.Before(phaseEndsAt) {
					break
				}
			}
			recordIndex := randomGenerator.Intn(workloadConfig.Records)
			if workloadConfig.Distribution == "hot" && randomGenerator.Intn(100) < 80 {
				recordIndex = hotRecordIndexes[randomGenerator.Intn(len(hotRecordIndexes))]
			}
			readOnly := randomGenerator.Intn(100) < workloadConfig.ReadPercent
			select {
			case workerSlots <- struct{}{}:
				workloadWorkers.Add(1)
				active.Inc()
				go func(phaseName string, recordIndex int, readOnly bool) {
					defer workloadWorkers.Done()
					defer func() { <-workerSlots; active.Dec() }()
					command := dto.DocumentGetRequest{PartitionKey: keys[recordIndex], ID: "record"}
					documentRevision, code := sendDocumentCommand(phaseName, "get", command)
					if !readOnly && code == 200 {
						update := dto.DocumentUpdateRequest{PartitionKey: keys[recordIndex], ID: "record", Payload: payload, ExpectedRevision: documentRevision}
						sendDocumentCommand(phaseName, "update", update)
					}
				}(phase.name, recordIndex, readOnly)
			default:
				drops.WithLabelValues(phase.name).Inc()
				if phase.name == "measure" {
					dropped.Add(1)
				}
			}
			nextIterationAt = nextIterationAt.Add(interval)
		}
		workloadWorkers.Wait()
		if phase.name == "measure" {
			runSummary.MeasurementFinished = time.Now().UTC()
		}
	}
	offered.Set(0)
	setPhase("finished")
	runSummary.Finished = time.Now().UTC()
	runSummary.Dropped = dropped.Load()
	sort.Float64s(latencySamples)
	quantile := func(quantile float64) float64 {
		if len(latencySamples) == 0 {
			return 0
		}
		return latencySamples[int(quantile*float64(len(latencySamples)-1))]
	}
	runSummary.P50Seconds = quantile(.5)
	runSummary.P95Seconds = quantile(.95)
	runSummary.P99Seconds = quantile(.99)
	if operationContext.Err() != nil {
		return operationContext.Err()
	}
	if writeErr != nil {
		return writeErr
	}
	if err := buffer.Flush(); err != nil {
		return err
	}
	if err := requestsFile.Close(); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(runSummary, "", "  ")
	if operationError := os.WriteFile(filepath.Join(outputDirectory, "summary.json"), data, 0644); operationError != nil {
		return operationError
	}
	metrics, err := snapshotMetrics(registry)
	if err != nil {
		return err
	}
	if operationError := os.WriteFile(filepath.Join(outputDirectory, "metrics.prom"), metrics, 0644); operationError != nil {
		return operationError
	}
	fmt.Printf("COMPLETE %s\n%s\n", outputDirectory, data)
	if workloadConfig.Hold {
		<-operationContext.Done()
	}
	return nil
}

// main разбирает аргументы запуска и завершает процесс с ненулевым кодом при ошибке приложения.
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
