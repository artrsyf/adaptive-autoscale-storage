package service

import "fmt"

type ProcessingUnitServiceConfig struct {
	// MaxInflight — Максимум одновременно исполняемых команд на процесс; лишние сразу получают HTTP 429.
	MaxInflight int `yaml:"max_inflight"`
}

// Validate проверяет admission; актуальные topology metadata принадлежат PostgreSQL-backed assignment.
func (processingUnitServiceConfig ProcessingUnitServiceConfig) Validate() error {
	if processingUnitServiceConfig.MaxInflight < 1 {
		return fmt.Errorf("invalid processing configuration")
	}
	return nil
}
