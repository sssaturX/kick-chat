package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port         string
	DatabaseURL  string
	RedisURL     string
	HMACSecret   string
	AdminAPIKey  string
	RateLimitRPS int
}

func Load() (*Config, error) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379/0"
	}
	hmacSecret := os.Getenv("HMAC_SECRET")
	if hmacSecret == "" {
		return nil, fmt.Errorf("HMAC_SECRET is required")
	}
	adminKey := strings.TrimSpace(os.Getenv("ADMIN_API_KEY"))
	if adminKey == "" {
		return nil, fmt.Errorf("ADMIN_API_KEY is required")
	}
	rps := 100
	if v := os.Getenv("RATE_LIMIT_RPS"); v != "" {
		if n, err := fmt.Sscanf(v, "%d", &rps); err != nil || n != 1 || rps <= 0 {
			rps = 100
		}
	}
	return &Config{
		Port:         port,
		DatabaseURL:  dbURL,
		RedisURL:     redisURL,
		HMACSecret:   hmacSecret,
		AdminAPIKey:  adminKey,
		RateLimitRPS: rps,
	}, nil
}
