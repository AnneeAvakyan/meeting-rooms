package config

import (
	"os"
	"time"
)

type Config struct {
	Port        string
	DatabaseURL string
	Secret      []byte
	TimeToLive  time.Duration
}

func Load() Config {
	return Config{
		Port:        getEnv("APP_PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/service?sslmode=disable"),
		Secret:      []byte(os.Getenv("SECRET_KEY")),
		TimeToLive:  3600 * time.Second,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
