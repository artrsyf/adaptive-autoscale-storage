// Package app composes the processing-unit application. It has no routing client.
package app

import (
	"context"
	"fmt"
	"os"

	"autoscale-distr-storage/processing-unit/internal/document/repository/postgres"
	documents "autoscale-distr-storage/processing-unit/internal/document/service"
	api "autoscale-distr-storage/processing-unit/internal/document/transport/http"
	"autoscale-distr-storage/processing-unit/internal/processingunit/metrics"
	"autoscale-distr-storage/processing-unit/internal/processingunit/service"
	"autoscale-distr-storage/processing-unit/internal/utils"
	"github.com/prometheus/client_golang/prometheus"
)

// Run собирает зависимости processing unit, подключает PostgreSQL и запускает HTTP-сервер.
func Run(configPath, nodeID string) error {
	var processingUnitConfig ProcessingUnitConfig
	if err := utils.LoadYAML(configPath, &processingUnitConfig); err != nil {
		return err
	}
	if err := processingUnitConfig.Validate(); err != nil {
		return err
	}
	postgresPassword := os.Getenv("POSTGRES_PASSWORD")
	if postgresPassword == "" {
		return fmt.Errorf("POSTGRES_PASSWORD is required")
	}
	metricsRegistry := prometheus.NewRegistry()
	metricsRegistry.MustRegister(prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	documentRepository, err := postgres.Open(context.Background(), processingUnitConfig.Postgres, postgresPassword, nodeID, metricsRegistry)
	if err != nil {
		return fmt.Errorf("initialize PostgreSQL adapter: invalid connection settings")
	}
	defer documentRepository.Close()
	partitionRequestMetrics := metrics.NewPartitionRequestMetrics(metricsRegistry, nodeID)
	processingUnitService, err := service.NewProcessingUnitService(processingUnitConfig.Processing, nodeID, documents.NewDocumentService(documentRepository), partitionRequestMetrics)
	if err != nil {
		return err
	}
	documentHttpRoutes := api.NewDocumentHttpRoutes(processingUnitConfig.API, processingUnitService, api.NewDocumentHttpMetrics(metricsRegistry, "processing-unit", nodeID))
	return utils.ServeHTTP(processingUnitConfig.Server, documentHttpRoutes, metricsRegistry, documentRepository.Ready)
}
