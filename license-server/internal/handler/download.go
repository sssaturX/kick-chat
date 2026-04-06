package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	_ "embed"
	"net/http"
	"os"
	"strings"
	"time"

	"license-server/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

//go:embed download_portal.html
var downloadPortalHTML []byte

const downloadRedisPrefix = "download:tok:"
const downloadTokenTTL = 4 * time.Minute

type DownloadHandler struct {
	svc      *service.LicenseService
	rdb      *redis.Client
	filePath string
	fileName string
	log      *zap.Logger
}

func NewDownloadHandler(svc *service.LicenseService, rdb *redis.Client, filePath, attachFileName string, log *zap.Logger) *DownloadHandler {
	if strings.TrimSpace(attachFileName) == "" {
		attachFileName = "SaturX.zip"
	}
	return &DownloadHandler{
		svc:      svc,
		rdb:      rdb,
		filePath: strings.TrimSpace(filePath),
		fileName: attachFileName,
		log:      log,
	}
}

func (h *DownloadHandler) PortalPage(c *gin.Context) {
	c.Header("Referrer-Policy", "no-referrer")
	c.Data(http.StatusOK, "text/html; charset=utf-8", downloadPortalHTML)
}

type downloadVerifyRequest struct {
	LicenseKey string `json:"license_key" binding:"required"`
}

func (h *DownloadHandler) Verify(c *gin.Context) {
	if h.filePath == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "downloads_not_configured"})
		return
	}
	if _, err := os.Stat(h.filePath); err != nil {
		h.log.Warn("download file missing", zap.String("path", h.filePath), zap.Error(err))
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "file_unavailable"})
		return
	}

	var req downloadVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	key := strings.TrimSpace(req.LicenseKey)
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	status, _, err := h.svc.Validate(c.Request.Context(), key, "")
	if err != nil {
		h.log.Error("download verify validate", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if status != "active" {
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid_license", "status": status})
		return
	}

	tok, err := randomHex(24)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	rkey := downloadRedisPrefix + tok
	if err := h.rdb.Set(c.Request.Context(), rkey, "1", downloadTokenTTL).Err(); err != nil {
		h.log.Error("download verify redis", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"download_path": "/download/file?token=" + tok,
	})
}

func (h *DownloadHandler) ServeFile(c *gin.Context) {
	if h.filePath == "" {
		c.Status(http.StatusNotFound)
		return
	}
	tok := strings.TrimSpace(c.Query("token"))
	if tok == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing token"})
		return
	}
	rkey := downloadRedisPrefix + tok
	ctx := context.Background()
	n, err := h.rdb.Del(ctx, rkey).Result()
	if err != nil || n == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid_or_expired_token"})
		return
	}
	c.Header("Referrer-Policy", "no-referrer")
	c.FileAttachment(h.filePath, h.fileName)
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
