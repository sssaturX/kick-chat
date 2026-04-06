package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	Port         string
	DatabaseURL  string
	RedisURL     string
	HMACSecret   string
	AdminAPIKey  string
	RateLimitRPS int

	// Optional: gated download portal at GET /download (requires Redis for one-time tokens)
	DownloadFilePath string // host path to zip/installer
	DownloadFileName string // filename shown in browser (Content-Disposition)

	AdminSessionTTL   time.Duration // Redis-backed admin UI session cookie
	AdminCookieSecure bool          // Set Secure flag on session cookie (use true behind HTTPS)
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
	dlPath := strings.TrimSpace(os.Getenv("DOWNLOAD_FILE_PATH"))
	dlName := strings.TrimSpace(os.Getenv("DOWNLOAD_FILE_NAME"))
	if dlName == "" && dlPath != "" {
		dlName = "SaturX.zip"
	}

	sessionTTL := 24 * time.Hour
	if v := strings.TrimSpace(os.Getenv("ADMIN_SESSION_TTL")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			sessionTTL = d
		}
	}
	cookieSecure := strings.EqualFold(os.Getenv("ADMIN_COOKIE_SECURE"), "true") || os.Getenv("ADMIN_COOKIE_SECURE") == "1"

	return &Config{
		Port:              port,
		DatabaseURL:       dbURL,
		RedisURL:          redisURL,
		HMACSecret:        hmacSecret,
		AdminAPIKey:       adminKey,
		RateLimitRPS:      rps,
		DownloadFilePath:  dlPath,
		DownloadFileName:  dlName,
		AdminSessionTTL:   sessionTTL,
		AdminCookieSecure: cookieSecure,
	}, nil
}
