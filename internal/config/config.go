package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port           string
	MongoURI       string
	DatabaseName   string
	JWTSecret      string
	AllowedOrigins []string
	RequestTimeout time.Duration
	MaxUploadBytes int64
}

func Load() (Config, error) {
	cfg := Config{
		Port:           envOr("PORT", "8080"),
		MongoURI:       os.Getenv("MONGODB_URI"),
		DatabaseName:   envOr("MONGODB_DATABASE", "mereader"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		AllowedOrigins: splitAndTrim(envOr("ALLOWED_ORIGINS", "*")),
		RequestTimeout: 60 * time.Second,
		MaxUploadBytes: int64(numberOr("MAX_UPLOAD_MB", 80)) << 20,
	}

	if cfg.MongoURI == "" {
		return Config{}, errors.New("MONGODB_URI is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return Config{}, errors.New("JWT_SECRET is required and must be at least 32 characters")
	}

	return cfg, nil
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func numberOr(key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func splitAndTrim(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}
