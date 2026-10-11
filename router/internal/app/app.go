// Package app composes the routing application. It has no database dependency.
package app

import (
	assignmentclient "autoscale-distr-storage/router/internal/assignment/client/http"
	assignmentservice "autoscale-distr-storage/router/internal/assignment/service"
	"context"
	"net/http"
	"time"

	"autoscale-distr-storage/router/internal/document/service"
	api "autoscale-distr-storage/router/internal/document/transport/http"
	"autoscale-distr-storage/router/internal/processingunit/client/http"
	"autoscale-distr-storage/router/internal/utils"
	"github.com/prometheus/client_golang/prometheus"
)

// Run собирает зависимости Router, подключает исходящий клиент и запускает HTTP-сервер.
func Run(configPath string) error {
	var routerConfig RouterConfig
	if err := utils.LoadYAML(configPath, &routerConfig); err != nil {
		return err
	}
	if err := routerConfig.Validate(); err != nil {
		return err
	}
	metricsRegistry := prometheus.NewRegistry()
	metricsRegistry.MustRegister(prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	assignmentRefreshes := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "storage_assignment_refresh_total", Help: "Assignment snapshot refresh attempts by outcome."}, []string{"outcome"})
	metricsRegistry.MustRegister(assignmentRefreshes)
	processingUnitHttpClient := &httpclient.ProcessingUnitHttpClient{Client: routerConfig.ProcessingUnitClient.NewClient()}
	dynamicAssignmentService := assignmentservice.NewDynamicAssignmentService()
	controlPlaneClient := &assignmentclient.ControlPlaneHttpClient{BaseURL: routerConfig.Assignment.ControlPlaneURL, Client: &http.Client{Timeout: routerConfig.Assignment.RequestTimeout.Time()}}
	initialContext, cancelInitial := context.WithTimeout(context.Background(), routerConfig.Assignment.RequestTimeout.Time())
	initialSnapshot, err := controlPlaneClient.Current(initialContext)
	cancelInitial()
	if err != nil {
		assignmentRefreshes.WithLabelValues("error").Inc()
		return err
	}
	if err = dynamicAssignmentService.Install(initialSnapshot); err != nil {
		assignmentRefreshes.WithLabelValues("invalid").Inc()
		return err
	}
	assignmentRefreshes.WithLabelValues("success").Inc()
	refreshContext, stopRefresh := context.WithCancel(context.Background())
	defer stopRefresh()
	go func() {
		refreshTicker := time.NewTicker(routerConfig.Assignment.RefreshInterval.Time())
		defer refreshTicker.Stop()
		for {
			select {
			case <-refreshContext.Done():
				return
			case <-refreshTicker.C:
				requestContext, cancelRequest := context.WithTimeout(refreshContext, routerConfig.Assignment.RequestTimeout.Time())
				snapshot, refreshError := controlPlaneClient.Current(requestContext)
				cancelRequest()
				if refreshError == nil {
					if installError := dynamicAssignmentService.Install(snapshot); installError == nil {
						assignmentRefreshes.WithLabelValues("success").Inc()
					} else {
						assignmentRefreshes.WithLabelValues("invalid").Inc()
					}
				} else {
					assignmentRefreshes.WithLabelValues("error").Inc()
				}
			}
		}
	}()
	documentRoutingService, err := service.NewDocumentRoutingService(routerConfig.Routing, dynamicAssignmentService, processingUnitHttpClient)
	if err != nil {
		return err
	}
	metricsRegistry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "storage_topology_nodes", Help: "Active assignment node count."}, func() float64 { return float64(len(dynamicAssignmentService.Nodes())) }))
	metricsRegistry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "storage_assignment_epoch", Help: "Active assignment epoch."}, func() float64 { return float64(dynamicAssignmentService.Epoch()) }))
	documentHttpRoutes := api.NewDocumentHttpRoutes(routerConfig.API, documentRoutingService, api.NewDocumentHttpMetrics(metricsRegistry, "router", "router"))
	return utils.ServeHTTP(routerConfig.Server, documentHttpRoutes, metricsRegistry, func(operationContext context.Context) error {
		return processingUnitHttpClient.Ready(operationContext, dynamicAssignmentService.Nodes())
	})
}
