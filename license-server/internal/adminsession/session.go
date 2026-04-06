package adminsession

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/redis/go-redis/v9"
)

const CookieName = "admin_session"

const redisPrefix = "admin:sess:"

// NewToken returns a random opaque session token (hex).
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Store saves the session in Redis with TTL.
func Store(ctx context.Context, client *redis.Client, token string, ttl time.Duration) error {
	return client.Set(ctx, redisPrefix+token, "1", ttl).Err()
}

// Valid reports whether the session token exists in Redis.
func Valid(ctx context.Context, client *redis.Client, token string) bool {
	if token == "" {
		return false
	}
	n, err := client.Exists(ctx, redisPrefix+token).Result()
	return err == nil && n > 0
}

// Delete removes the session from Redis.
func Delete(ctx context.Context, client *redis.Client, token string) error {
	return client.Del(ctx, redisPrefix+token).Err()
}
