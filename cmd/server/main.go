// Command server запускает HTTP-сервер сервиса бронирования переговорных комнат.
package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/AnneeAvakyan/meeting-rooms/internal/config"
	"github.com/AnneeAvakyan/meeting-rooms/internal/httpapi"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := config.Load()

	srv := &httpapi.Server{}

	addr := ":" + cfg.Port
	logger.Info("starting server", "addr", addr)
	if err := http.ListenAndServe(addr, srv.NewRouter()); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
