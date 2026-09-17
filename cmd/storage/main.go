package main

import (
	"autoscale-distr-storage/internal/app"
	"errors"
	"log/slog"
	"net/http"
	"os"
)

func main() {
	if err := app.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("service failed", "error", err)
		os.Exit(1)
	}
}
