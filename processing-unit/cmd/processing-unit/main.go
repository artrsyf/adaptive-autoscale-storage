package main

import (
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"

	"autoscale-distr-storage/processing-unit/internal/app"
)

// main разбирает аргументы запуска и завершает процесс с ненулевым кодом при ошибке приложения.
func main() {
	path := flag.String("config", "config.yaml", "YAML configuration path")
	node := flag.String("node", "processing-unit-1", "processing unit identity")
	flag.Parse()
	if err := app.Run(*path, *node); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("service failed", "error", err)
		os.Exit(1)
	}
}
