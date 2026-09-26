package main

import (
	"encoding/json"
	"fmt"
	"go.yaml.in/yaml/v2"
	"io"
	"net/url"
	"os"
	"time"
)

type duration time.Duration

// MarshalJSON сохраняет длительность строкой с единицами в отчёте о параметрах нагрузки.
func (configuredDuration duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(configuredDuration).String())
}

// UnmarshalYAML читает длительность из YAML-строки с единицами, например 2500ms или 3s.
func (configuredDuration *duration) UnmarshalYAML(unmarshal func(any) error) error {
	var value string
	if err := unmarshal(&value); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(value)
	*configuredDuration = duration(parsed)
	return err
}

// This is the workload client's own configuration contract, not a server import.
type LoadConfig struct {
	// Target — Базовый HTTP-адрес Router, доступный из контейнера генератора.
	Target string `json:"target" yaml:"target"`
	// Scenario — Профиль темпа: constant — постоянный, ramp — рост, burst — кратковременный всплеск.
	Scenario string `json:"scenario" yaml:"scenario"`
	// Distribution — uniform — равномерный выбор ключей; hot — 80% итераций на ключи раздела 0.
	Distribution string `json:"distribution" yaml:"distribution"`
	// Rate — Целевые логические итерации в секунду, не HTTP RPS: update включает get и update.
	Rate int `json:"rate" yaml:"rate"`
	// ReadPercent — Доля read-only итераций в процентах (0–100); остальные читают и условно обновляют.
	ReadPercent int `json:"read_percent" yaml:"read_percent"`
	// Records — Число документов в наборе генератора, не менее 128; заполнение не входит в измерение.
	Records int `json:"records" yaml:"records"`
	// PayloadBytes — Приблизительный размер JSON payload в байтах (16–900000).
	PayloadBytes int `json:"payload_bytes" yaml:"payload_bytes"`
	// Workers — Предел параллельных итераций (1–4096); при насыщении новые итерации учитываются как пропущенные.
	Workers int `json:"workers" yaml:"workers"`
	// Seed — Начальное значение генератора случайных чисел для воспроизводимого выбора запросов.
	Seed int64 `json:"seed" yaml:"seed"`
	// Duration — Длительность измеряемой фазы с единицами; заполнение и прогрев выполняются отдельно.
	Duration duration `json:"duration" yaml:"duration"`
	// Warmup — Длительность прогрева перед измерением; 0s отключает прогрев.
	Warmup duration `json:"warmup" yaml:"warmup"`
	// RequestTimeout — Предельное время одного HTTP-вызова генератора, включая подключение и чтение ответа.
	RequestTimeout duration `json:"request_timeout" yaml:"request_timeout"`
	// MetricsPort — TCP-порт endpoint /metrics генератора; должен совпадать с target Prometheus.
	MetricsPort int `json:"metrics_port" yaml:"metrics_port"`
	// Partitions — Число логических разделов (1–1024); должно совпадать у Router, исполнителей и генератора нагрузки.
	Partitions int `json:"partitions" yaml:"partitions"`
	// Output — Каталог результатов; каждый прогон создаёт вложенную директорию с UTC-временем.
	Output string `json:"output" yaml:"output"`
	// Hold — Оставлять генератор работающим после прогона, чтобы Prometheus успел собрать итоговые метрики.
	Hold bool `json:"hold" yaml:"hold"`
}

// loadConfig читает YAML нагрузки и проверяет адрес сервера, порт метрик и параметры выполнения.
func loadConfig(path string) (LoadConfig, error) {
	var loadConfig LoadConfig
	configFile, operationError := os.Open(path)
	if operationError != nil {
		return loadConfig, operationError
	}
	defer configFile.Close()
	decoder := yaml.NewDecoder(configFile)
	decoder.SetStrict(true)
	if operationError = decoder.Decode(&loadConfig); operationError != nil {
		return loadConfig, operationError
	}
	if decoder.Decode(new(any)) != io.EOF {
		return loadConfig, fmt.Errorf("multiple configuration values")
	}
	parsedURL, operationError := url.Parse(loadConfig.Target)
	if operationError != nil || parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.User != nil || loadConfig.Partitions < 1 || loadConfig.Partitions > 1024 || loadConfig.MetricsPort < 1 || loadConfig.MetricsPort > 65535 || loadConfig.RequestTimeout <= 0 || loadConfig.Output == "" {
		return loadConfig, fmt.Errorf("invalid workload endpoint or runtime settings")
	}
	return loadConfig, nil
}
