package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type RateLimiter struct {
	client *redis.Client
	rps    int
	log    *zap.Logger
}

func NewRateLimiter(client *redis.Client, rps int, log *zap.Logger) *RateLimiter {
	if rps <= 0 {
		rps = 100
	}
	return &RateLimiter{client: client, rps: rps, log: log}
}

func (r *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := "rl:" + c.ClientIP()
		ctx := c.Request.Context()
		count, err := r.client.Incr(ctx, key).Result()
		if err != nil {
			r.log.Warn("rate limit incr", zap.Error(err))
			c.Next()
			return
		}
		if count == 1 {
			r.client.Expire(ctx, key, time.Second)
		}
		if count > int64(r.rps) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}
