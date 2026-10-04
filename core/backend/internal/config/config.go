package config

import (
	"os"
)

type Config struct {
	DSN  string
	Port string
}

func Getenv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func Load() *Config {

	return &Config{
		DSN:  Getenv("DSN", "postgres://postgres:password@localhost:5432/your_db_name?sslmode=disable"),
		Port: Getenv("PORT", "2026"),
	}
}
