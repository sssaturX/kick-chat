package middleware

import (
	"fmt"
	"time"

	"license-server/internal/logbuffer"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func Logger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		clientIP := c.ClientIP()
		method := c.Request.Method
		c.Next()
		latency := time.Since(start)
		status := c.Writer.Status()
		log.Info("request",
			zap.Int("status", status),
			zap.Duration("latency", latency),
			zap.String("method", method),
			zap.String("path", path),
			zap.String("ip", clientIP),
		)
		if buf, has := c.Get("log_buffer"); has && buf != nil {
			if b, ok := buf.(*logbuffer.Buffer); ok {
				b.Append(fmt.Sprintf("%s %s %s %d %s", clientIP, method, path, status, latency))
			}
		}
	}
}

// RequestLogToBuffer stores each request summary in the given buffer for admin logs view.
func RequestLogToBuffer(b *logbuffer.Buffer) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("log_buffer", b)
		c.Next()
	}
}
