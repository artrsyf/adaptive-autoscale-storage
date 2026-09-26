package api

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

// DocumentHttpMiddleware применяет общие лимиты и наблюдаемость ко всем документным маршрутам.
type DocumentHttpMiddleware struct {
	Config  DocumentHttpConfig
	Metrics *DocumentHttpMetrics
}

// documentStatusWriter сохраняет фактически отправленный статус для метрик.
type documentStatusWriter struct {
	http.ResponseWriter
	status int
}

// WriteHeader записывает только первый статус ответа.
func (statusWriter *documentStatusWriter) WriteHeader(status int) {
	if statusWriter.status != 0 {
		return
	}
	statusWriter.status = status
	statusWriter.ResponseWriter.WriteHeader(status)
}

// Write устанавливает статус 200 при неявной записи тела ответа.
func (statusWriter *documentStatusWriter) Write(body []byte) (int, error) {
	if statusWriter.status == 0 {
		statusWriter.WriteHeader(http.StatusOK)
	}
	return statusWriter.ResponseWriter.Write(body)
}

// Wrap ограничивает тело и контекст запроса, записывает ошибки и метрики с фиксированной меткой операции.
// Deadline кооперативный: обработчик и зависимости должны учитывать отмену контекста.
func (documentHttpMiddleware DocumentHttpMiddleware) Wrap(operation string, next func(http.ResponseWriter, *http.Request) error) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, httpRequest *http.Request) {
		start := time.Now()
		tracked := &documentStatusWriter{ResponseWriter: responseWriter}
		defer func() {
			status := tracked.status
			if status == 0 {
				status = http.StatusOK
			}
			documentHttpMiddleware.Metrics.Requests.WithLabelValues(operation, strconv.Itoa(status)).Inc()
			documentHttpMiddleware.Metrics.Latency.WithLabelValues(operation).Observe(time.Since(start).Seconds())
		}()
		operationContext, cancel := context.WithTimeout(httpRequest.Context(), documentHttpMiddleware.Config.RequestTimeout.Time())
		defer cancel()
		httpRequest = httpRequest.WithContext(operationContext)
		httpRequest.Body = http.MaxBytesReader(tracked, httpRequest.Body, documentHttpMiddleware.Config.MaxBodyBytes)
		if err := next(tracked, httpRequest); err != nil {
			writeDocumentFailure(tracked, err)
		}
	})
}
