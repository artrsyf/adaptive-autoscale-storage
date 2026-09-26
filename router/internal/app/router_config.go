package app

import (
	"fmt"

	assignmentservice "autoscale-distr-storage/router/internal/assignment/service"
	"autoscale-distr-storage/router/internal/document/service"
	api "autoscale-distr-storage/router/internal/document/transport/http"
	"autoscale-distr-storage/router/internal/processingunit/client/http"
	"autoscale-distr-storage/router/internal/utils"
)

type RouterConfig struct {
	// Server — Жизненный цикл входящего HTTP-сервера, включая health и /metrics.
	Server utils.ServerConfig `yaml:"server"`
	// API — Ограничения документного HTTP API.
	API api.DocumentHttpConfig `yaml:"api"`
	// Routing — Ограничение одновременных запросов, направляемых Router исполнителям.
	Routing service.DocumentRoutingConfig `yaml:"routing"`
	// ProcessingUnitClient — Соединения и таймауты исходящего HTTP-клиента Router.
	ProcessingUnitClient httpclient.ProcessingUnitHttpClientConfig `yaml:"processing_unit_client"`
	// Assignment — Статическая карта маршрутизации; при изменении требуется согласованное обновление стенда.
	Assignment assignmentservice.AssignmentConfig `yaml:"assignment"`
}

// Validate проверяет конфиги компонентов и требует write timeout больше deadline API.
func (routerConfig RouterConfig) Validate() error {
	if err := routerConfig.Server.Validate(); err != nil {
		return err
	}
	if err := routerConfig.API.Validate(); err != nil {
		return err
	}
	if routerConfig.Server.WriteTimeout <= routerConfig.API.RequestTimeout {
		return fmt.Errorf("write timeout must exceed request timeout")
	}
	if err := routerConfig.ProcessingUnitClient.Validate(); err != nil {
		return err
	}
	return routerConfig.Assignment.Validate()
}
