package main

import (
	"testing"
	"time"
)

// TestLoadYAML проверяет чтение поставляемого YAML нагрузки и преобразование длительностей.
func TestLoadYAML(test *testing.T) {
	workloadConfig, err := loadConfig("config.yaml")
	if err != nil {
		test.Fatal(err)
	}
	if workloadConfig.Target != "http://router:8080" || time.Duration(workloadConfig.Duration) != time.Minute || workloadConfig.Partitions != 128 {
		test.Fatal("unexpected workload configuration")
	}
}
