package app

import (
	"testing"

	"autoscale-distr-storage/processing-unit/internal/utils"
)

// TestConfiguration проверяет чтение и валидацию поставляемого конфига приложения.
func TestConfiguration(test *testing.T) {
	var processingUnitConfig ProcessingUnitConfig
	if err := utils.LoadYAML("../../config.yaml", &processingUnitConfig); err != nil {
		test.Fatal(err)
	}
	if err := processingUnitConfig.Validate(); err != nil {
		test.Fatal(err)
	}
}
