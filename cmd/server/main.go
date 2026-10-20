// Command server запускает HTTP-сервер сервиса бронирования переговорных комнат.
package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/vv/meeting-room-booking/internal/config"
	"github.com/vv/meeting-room-booking/internal/httpapi"
	"github.com/vv/meeting-room-booking/internal/storage"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := config.Load()

	// На старте используется in-memory хранилище (см. internal/storage/memory.go).
	// TODO: заменить на PostgreSQL-репозиторий, реализующий те же интерфейсы
	// storage.RoomRepository / storage.BookingRepository, когда будет готова БД.
	store := storage.NewMemoryStore()

	srv := &httpapi.Server{
		Rooms:    store,
		Bookings: store,
		Logger:   logger,
	}

	addr := ":" + cfg.Port
	logger.Info("starting server", "addr", addr)
	if err := http.ListenAndServe(addr, srv.NewRouter()); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
