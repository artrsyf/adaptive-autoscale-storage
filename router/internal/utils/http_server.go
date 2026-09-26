package utils

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ServerConfig configures HTTP lifecycle only; it knows nothing about Processing unit or Router roles.
type ServerConfig struct {
	// ListenHost — Адрес прослушивания; 0.0.0.0 принимает подключения на всех интерфейсах контейнера.
	ListenHost string `yaml:"listen_host"`
	// Port — TCP-порт сервера; допустимы значения от 1 до 65535.
	Port int `yaml:"port"`
	// ReadHeaderTimeout — Предельное время чтения HTTP-заголовков, например 2s.
	ReadHeaderTimeout Duration `yaml:"read_header_timeout"`
	// ReadTimeout — Предельное время чтения всего HTTP-запроса, включая тело.
	ReadTimeout Duration `yaml:"read_timeout"`
	// WriteTimeout — Предельное время записи HTTP-ответа; должно превышать request_timeout API.
	WriteTimeout Duration `yaml:"write_timeout"`
	// IdleTimeout — Время ожидания следующего запроса в неактивном keep-alive соединении.
	IdleTimeout Duration `yaml:"idle_timeout"`
	// ReadyTimeout — Deadline одной проверки готовности: БД у исполнителя, всех нод у Router.
	ReadyTimeout Duration `yaml:"ready_timeout"`
	// ShutdownTimeout — Время ожидания активных HTTP-запросов при завершении процесса.
	ShutdownTimeout Duration `yaml:"shutdown_timeout"`
}

// Validate проверяет адрес прослушивания, диапазон порта и положительные HTTP-таймауты.
func (serverConfig ServerConfig) Validate() error {
	if serverConfig.ListenHost == "" || serverConfig.Port < 1 || serverConfig.Port > 65535 {
		return fmt.Errorf("invalid HTTP listener")
	}
	for _, configuredDuration := range []Duration{serverConfig.ReadHeaderTimeout, serverConfig.ReadTimeout, serverConfig.WriteTimeout, serverConfig.IdleTimeout, serverConfig.ReadyTimeout, serverConfig.ShutdownTimeout} {
		if configuredDuration <= 0 {
			return fmt.Errorf("HTTP lifecycle timeouts must be positive")
		}
	}
	return nil
}

// ServeHTTP подключает API, health и метрики; при остановке ждёт активные запросы до shutdown timeout.
func ServeHTTP(serverConfig ServerConfig, api http.Handler, registry *prometheus.Registry, ready func(context.Context) error) error {
	operationContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	httpMux := http.NewServeMux()
	httpMux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	httpMux.HandleFunc("/livez", func(responseWriter http.ResponseWriter, httpRequest *http.Request) { responseWriter.WriteHeader(200) })
	httpMux.HandleFunc("/readyz", func(responseWriter http.ResponseWriter, httpRequest *http.Request) {
		if operationContext.Err() != nil {
			http.Error(responseWriter, `{"error":"draining"}`, 503)
			return
		}
		check, cancel := context.WithTimeout(httpRequest.Context(), serverConfig.ReadyTimeout.Time())
		defer cancel()
		if ready(check) != nil {
			http.Error(responseWriter, `{"error":"not_ready"}`, 503)
			return
		}
		responseWriter.WriteHeader(200)
	})
	httpMux.Handle("/", api)
	server := &http.Server{Addr: net.JoinHostPort(serverConfig.ListenHost, strconv.Itoa(serverConfig.Port)), Handler: httpMux, ReadHeaderTimeout: serverConfig.ReadHeaderTimeout.Time(), ReadTimeout: serverConfig.ReadTimeout.Time(), WriteTimeout: serverConfig.WriteTimeout.Time(), IdleTimeout: serverConfig.IdleTimeout.Time()}
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	select {
	case err := <-done:
		return err
	case <-operationContext.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), serverConfig.ShutdownTimeout.Time())
	defer cancel()
	if err := server.Shutdown(shutdown); err != nil {
		_ = server.Close()
		return err
	}
	return nil
}
