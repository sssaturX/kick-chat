package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const AdminAPIKeyHeader = "X-Admin-API-Key"

func AdminAuth(apiKey string) gin.HandlerFunc {
	apiKey = strings.TrimSpace(apiKey)
	return func(c *gin.Context) {
		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "ADMIN_API_KEY not configured"})
			return
		}
		key := strings.TrimSpace(c.GetHeader(AdminAPIKeyHeader))
		if key == "" && c.Request.Method == http.MethodGet && c.Query("admin_key") != "" {
			key = strings.TrimSpace(c.Query("admin_key"))
		}
		if key != apiKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or missing X-Admin-API-Key header"})
			return
		}
		c.Next()
	}
}
