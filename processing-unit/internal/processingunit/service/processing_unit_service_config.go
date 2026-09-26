package service

import "fmt"

type ProcessingUnitServiceConfig struct {
	// MaxInflight — Максимум одновременно исполняемых команд на процесс; лишние сразу получают HTTP 429.
	MaxInflight int `yaml:"max_inflight"`
	// PartitionCount — Допустимое число разделов (1–1024); должно совпадать с assignment.partitions Router.
	PartitionCount int `yaml:"partition_count"`
	// Epoch — Версия назначения; Router и processing unit должны использовать одинаковое значение. Это не fencing.
	Epoch string `yaml:"epoch"`
}

// Validate проверяет admission, непустую epoch и число разделов от 1 до 1024.
func (processingUnitServiceConfig ProcessingUnitServiceConfig) Validate() error {
	if processingUnitServiceConfig.MaxInflight < 1 || processingUnitServiceConfig.PartitionCount < 1 || processingUnitServiceConfig.PartitionCount > 1024 || processingUnitServiceConfig.Epoch == "" {
		return fmt.Errorf("invalid processing configuration")
	}
	return nil
}
