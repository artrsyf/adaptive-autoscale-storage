package app

import (
	"fmt"

	"autoscale-distr-storage/processing-unit/internal/document/repository/postgres"
	api "autoscale-distr-storage/processing-unit/internal/document/transport/http"
	"autoscale-distr-storage/processing-unit/internal/processingunit/service"
	"autoscale-distr-storage/processing-unit/internal/utils"
)

type ProcessingUnitConfig struct {
	// Server — Жизненный цикл входящего HTTP-сервера, включая health и /metrics.
	Server utils.ServerConfig `yaml:"server"`
	// API — Ограничения документного HTTP API.
	API api.DocumentHttpConfig `yaml:"api"`
	// Processing — Локальные ограничения исполнения; топология других нод здесь не хранится.
	Processing service.ProcessingUnitServiceConfig `yaml:"processing"`
	// Postgres — Подключение и пул PostgreSQL; пароль берётся отдельно из POSTGRES_PASSWORD.
	Postgres postgres.DocumentPostgresConfig `yaml:"postgres"`
}

// Validate проверяет конфиги компонентов и требует write timeout больше deadline API.
func (processingUnitConfig ProcessingUnitConfig) Validate() error {
	if err := processingUnitConfig.Server.Validate(); err != nil {
		return err
	}
	if err := processingUnitConfig.API.Validate(); err != nil {
		return err
	}
	if processingUnitConfig.Server.WriteTimeout <= processingUnitConfig.API.RequestTimeout {
		return fmt.Errorf("write timeout must exceed request timeout")
	}
	if err := processingUnitConfig.Postgres.Validate(); err != nil {
		return err
	}
	return processingUnitConfig.Processing.Validate()
}
