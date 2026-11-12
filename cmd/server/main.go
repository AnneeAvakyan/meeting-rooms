package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/AnneeAvakyan/meeting-rooms/internal/config"
	"github.com/AnneeAvakyan/meeting-rooms/internal/httpapi"
	"github.com/AnneeAvakyan/meeting-rooms/internal/repositoriesPostgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := config.Load()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	if err := pool.Ping(ctx); err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	roomRepository := repositoriesPostgres.NewRoomRepository(pool)
	bookingRepository := repositoriesPostgres.NewBookingRepositoryPostgres(pool)

	srv := &httpapi.Server{
		Rooms:    roomRepository,
		Bookings: bookingRepository,
		Logger:   logger,
	}

	addr := ":" + cfg.Port
	logger.Info("starting server", "addr", addr)
	if err := http.ListenAndServe(addr, srv.NewRouter()); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
