package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// LoginRateLimit limits POST /admin login attempts per client IP (anti brute-force).
func LoginRateLimit(client *redis.Client, max int, window time.Duration, log *zap.Logger) gin.HandlerFunc {
	if max <= 0 {
		max = 5
	}
	if window <= 0 {
		window = time.Minute
	}
	return func(c *gin.Context) {
		key := "rllogin:" + c.ClientIP()
		ctx := c.Request.Context()
		n, err := client.Incr(ctx, key).Result()
		if err != nil {
			log.Warn("login rate limit incr", zap.Error(err))
			c.Next()
			return
		}
		if n == 1 {
			client.Expire(ctx, key, window)
		}
		if n > int64(max) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many login attempts, try later"})
			return
		}
		c.Next()
	}
}
