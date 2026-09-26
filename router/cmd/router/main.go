package main

import (
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"

	"autoscale-distr-storage/router/internal/app"
)

// main разбирает аргументы запуска и завершает процесс с ненулевым кодом при ошибке приложения.
func main() {
	path := flag.String("config", "config.yaml", "YAML configuration path")
	flag.Parse()
	if err := app.Run(*path); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("service failed", "error", err)
		os.Exit(1)
	}
}
