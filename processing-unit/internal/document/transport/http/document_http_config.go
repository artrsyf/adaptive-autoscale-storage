package api

import (
	"autoscale-distr-storage/processing-unit/internal/utils"
	"fmt"
)

type DocumentHttpConfig struct {
	// MaxBodyBytes — Максимальный размер JSON-тела в байтах; превышение возвращает HTTP 413.
	MaxBodyBytes int64 `yaml:"max_body_bytes"`
	// RequestTimeout — Deadline обработки команды, включая ожидание зависимостей; длительность с единицами.
	RequestTimeout utils.Duration `yaml:"request_timeout"`
}

// Validate проверяет положительный лимит тела запроса и deadline API.
func (documentHttpConfig DocumentHttpConfig) Validate() error {
	if documentHttpConfig.MaxBodyBytes < 1 || documentHttpConfig.RequestTimeout <= 0 {
		return fmt.Errorf("invalid API size limit or timeout")
	}
	return nil
}
