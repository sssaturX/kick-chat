package handler

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	"license-server/internal/adminsession"
	"license-server/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type AdminUIHandler struct {
	svc          *service.LicenseService
	redis        *redis.Client
	apiKey       string
	sessionTTL   time.Duration
	cookieSecure bool
	log          *zap.Logger
}

func NewAdminUIHandler(svc *service.LicenseService, redis *redis.Client, apiKey string, sessionTTL time.Duration, cookieSecure bool, log *zap.Logger) *AdminUIHandler {
	return &AdminUIHandler{
		svc:          svc,
		redis:        redis,
		apiKey:       strings.TrimSpace(apiKey),
		sessionTTL:   sessionTTL,
		cookieSecure: cookieSecure,
		log:          log,
	}
}

type adminLoginBody struct {
	APIKey string `json:"api_key" binding:"required"`
}

func (h *AdminUIHandler) Login(c *gin.Context) {
	var body adminLoginBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	key := strings.TrimSpace(body.APIKey)
	if len(key) != len(h.apiKey) || subtle.ConstantTimeCompare([]byte(key), []byte(h.apiKey)) != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	token, err := adminsession.NewToken()
	if err != nil {
		h.log.Error("session token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if err := adminsession.Store(c.Request.Context(), h.redis, token, h.sessionTTL); err != nil {
		h.log.Error("session store", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	maxAge := int(h.sessionTTL.Seconds())
	if maxAge < 1 {
		maxAge = 3600
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(adminsession.CookieName, token, maxAge, "/", "", h.cookieSecure, true)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *AdminUIHandler) Logout(c *gin.Context) {
	tok, err := c.Cookie(adminsession.CookieName)
	if err == nil && tok != "" {
		_ = adminsession.Delete(c.Request.Context(), h.redis, tok)
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(adminsession.CookieName, "", -1, "/", "", h.cookieSecure, true)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *AdminUIHandler) ListLicenses(c *gin.Context) {
	list, err := h.svc.ListLicenses(c.Request.Context())
	if err != nil {
		h.log.Error("list licenses", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	out := make([]gin.H, 0, len(list))
	for _, lic := range list {
		out = append(out, gin.H{
			"id":                  lic.ID.String(),
			"license_key":         lic.LicenseKey,
			"status":              lic.Status,
			"expires_at":          lic.ExpiresAt.UTC().Format(time.RFC3339),
			"max_activations":     lic.MaxActivations,
			"current_activations": lic.CurrentActivations,
			"hwid":                lic.HWID,
			"created_at":          lic.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, gin.H{"licenses": out})
}

type adminDeleteBody struct {
	LicenseKey string `json:"license_key" binding:"required"`
}

func (h *AdminUIHandler) DeleteLicense(c *gin.Context) {
	var body adminDeleteBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	key := strings.TrimSpace(body.LicenseKey)
	if err := h.svc.AdminDelete(c.Request.Context(), key); err != nil {
		if errors.Is(err, service.ErrLicenseNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "license not found"})
			return
		}
		h.log.Error("admin delete", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
