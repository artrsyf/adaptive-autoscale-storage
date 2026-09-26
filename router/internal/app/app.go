// Package app composes the routing application. It has no database dependency.
package app

import (
	assignmentservice "autoscale-distr-storage/router/internal/assignment/service"
	"context"

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
	processingUnitHttpClient := &httpclient.ProcessingUnitHttpClient{Client: routerConfig.ProcessingUnitClient.NewClient()}
	staticAssignmentService, err := assignmentservice.NewStaticAssignmentService(routerConfig.Assignment)
	if err != nil {
		return err
	}
	documentRoutingService, err := service.NewDocumentRoutingService(routerConfig.Routing, staticAssignmentService, processingUnitHttpClient)
	if err != nil {
		return err
	}
	metricsRegistry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "storage_topology_nodes", Help: "Configured node count."}, func() float64 { return float64(len(routerConfig.Assignment.Nodes)) }))
	documentHttpRoutes := api.NewDocumentHttpRoutes(routerConfig.API, documentRoutingService, api.NewDocumentHttpMetrics(metricsRegistry, "router", "router"))
	return utils.ServeHTTP(routerConfig.Server, documentHttpRoutes, metricsRegistry, func(operationContext context.Context) error {
		return processingUnitHttpClient.Ready(operationContext, routerConfig.Assignment.Nodes)
	})
}
