package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"license-server/internal/adminsession"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const AdminAPIKeyHeader = "X-Admin-API-Key"

func apiKeyMatch(got, want string) bool {
	got = strings.TrimSpace(got)
	want = strings.TrimSpace(want)
	if len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// AdminAuthorized reports whether the request is authenticated as admin (API key or session cookie).
func AdminAuthorized(c *gin.Context, apiKey string, redis *redis.Client) bool {
	if strings.TrimSpace(apiKey) == "" {
		return false
	}
	if apiKeyMatch(c.GetHeader(AdminAPIKeyHeader), apiKey) {
		return true
	}
	tok, err := c.Cookie(adminsession.CookieName)
	if err != nil || tok == "" {
		return false
	}
	return adminsession.Valid(c.Request.Context(), redis, tok)
}

// AdminAuth requires X-Admin-API-Key or a valid admin_session cookie (Redis).
func AdminAuth(apiKey string, redis *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		if AdminAuthorized(c, apiKey, redis) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
	}
}

// AdminHTMLAuth redirects unauthenticated users to /admin/login (for GET dashboard).
func AdminHTMLAuth(apiKey string, redis *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		if AdminAuthorized(c, apiKey, redis) {
			c.Next()
			return
		}
		c.Redirect(http.StatusFound, "/admin/login")
		c.Abort()
	}
}
