package httpclient

import (
	"fmt"
	"net/http"

	"autoscale-distr-storage/router/internal/utils"
)

type ProcessingUnitHttpClientConfig struct {
	// MaxIdleConnections — Максимум неактивных HTTP-соединений во всём исходящем пуле Router.
	MaxIdleConnections int `yaml:"max_idle_connections"`
	// MaxIdlePerHost — Максимум неактивных HTTP-соединений к одному исполнителю.
	MaxIdlePerHost int `yaml:"max_idle_per_host"`
	// MaxConnectionsPerHost — Максимум всех HTTP-соединений к одному исполнителю; ожидание входит в deadline команды.
	MaxConnectionsPerHost int `yaml:"max_connections_per_host"`
	// IdleTimeout — Срок хранения неактивного соединения в исходящем HTTP-пуле.
	IdleTimeout utils.Duration `yaml:"idle_timeout"`
	// ResponseHeaderTimeout — Предел ожидания заголовков ответа исполнителя после отправки запроса.
	ResponseHeaderTimeout utils.Duration `yaml:"response_header_timeout"`
}

// NewClient создаёт HTTP-клиент с ограничениями соединений и без автоматического следования перенаправлениям.
func (processingUnitHttpClientConfig ProcessingUnitHttpClientConfig) NewClient() *http.Client {
	return &http.Client{Transport: &http.Transport{MaxIdleConns: processingUnitHttpClientConfig.MaxIdleConnections, MaxIdleConnsPerHost: processingUnitHttpClientConfig.MaxIdlePerHost, MaxConnsPerHost: processingUnitHttpClientConfig.MaxConnectionsPerHost, IdleConnTimeout: processingUnitHttpClientConfig.IdleTimeout.Time(), ResponseHeaderTimeout: processingUnitHttpClientConfig.ResponseHeaderTimeout.Time()}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

// Validate проверяет положительные лимиты соединений и таймауты исходящего HTTP-клиента.
func (processingUnitHttpClientConfig ProcessingUnitHttpClientConfig) Validate() error {
	if processingUnitHttpClientConfig.MaxIdleConnections < 1 || processingUnitHttpClientConfig.MaxIdlePerHost < 1 || processingUnitHttpClientConfig.MaxConnectionsPerHost < 1 || processingUnitHttpClientConfig.IdleTimeout <= 0 || processingUnitHttpClientConfig.ResponseHeaderTimeout <= 0 {
		return fmt.Errorf("invalid processing unit HTTP client limits or timeouts")
	}
	return nil
}
