package app

import (
	"testing"

	"autoscale-distr-storage/router/internal/utils"
)

// TestConfiguration проверяет чтение и валидацию поставляемого конфига приложения.
func TestConfiguration(test *testing.T) {
	var routerConfig RouterConfig
	if err := utils.LoadYAML("../../config.yaml", &routerConfig); err != nil {
		test.Fatal(err)
	}
	if err := routerConfig.Validate(); err != nil {
		test.Fatal(err)
	}
}
