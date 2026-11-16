package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	_ "github.com/AnneeAvakyan/meeting-rooms/docs"
	"github.com/AnneeAvakyan/meeting-rooms/internal/auth"
	"github.com/AnneeAvakyan/meeting-rooms/internal/config"
	"github.com/AnneeAvakyan/meeting-rooms/internal/httpapi"
	"github.com/AnneeAvakyan/meeting-rooms/internal/repositoriesPostgres"
	"github.com/AnneeAvakyan/meeting-rooms/internal/services"
	"github.com/jackc/pgx/v5/pgxpool"
)

// @title Meeting Rooms Booking API
// @version 1.0
// @description REST API для бронирования переговорных комнат.
// @description Поддерживает JWT-аутентификацию и роли employee/admin.
// @host localhost:8080
// @BasePath /
//
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Введите Bearer и JWT-токен, например: Bearer eyJ...
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
	userRepository := repositoriesPostgres.NewUserRepositoryPostgres(pool)
	authManager, err := auth.NewTokenManager(cfg.Secret, cfg.TimeToLive)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	userService := services.NewUserService(userRepository, authManager)
	authHandlers := httpapi.NewAuthHandlers(userService, authManager)

	srv := &httpapi.Server{
		Rooms:        roomRepository,
		Bookings:     bookingRepository,
		Users:        userRepository,
		Logger:       logger,
		AuthHandlers: authHandlers,
	}

	addr := ":" + cfg.Port
	logger.Info("starting server", "addr", addr)
	if err := http.ListenAndServe(addr, srv.NewRouter()); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
