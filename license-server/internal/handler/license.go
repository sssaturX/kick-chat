package handler

import (
	"errors"
	"net/http"
	"time"

	"license-server/internal/license"
	"license-server/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type LicenseHandler struct {
	svc    *service.LicenseService
	secret string
	log    *zap.Logger
}

func NewLicenseHandler(svc *service.LicenseService, secret string, log *zap.Logger) *LicenseHandler {
	return &LicenseHandler{svc: svc, secret: secret, log: log}
}

type ActivateRequest struct {
	LicenseKey        string `json:"license_key" binding:"required"`
	HWID              string `json:"hwid"`               // legacy
	DeviceFingerprint string `json:"device_fingerprint"`   // new: client sends raw fingerprint, server hashes
	AppVersion        string `json:"app_version"`
}

type ActivateResponse struct {
	Status        string `json:"status"`
	ExpiresAt     string `json:"expires_at"`
	ServerTime    string `json:"server_time"`
	Signature     string `json:"signature,omitempty"`     // legacy
	AccessToken   string `json:"access_token,omitempty"`
	RefreshToken  string `json:"refresh_token,omitempty"`
	SignedLicense string `json:"signed_license,omitempty"`
	DeviceID      string `json:"device_id,omitempty"`
}

func (h *LicenseHandler) Activate(c *gin.Context) {
	var req ActivateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	fp := req.DeviceFingerprint
	if fp == "" {
		fp = req.HWID
	}
	result, err := h.svc.Activate(c.Request.Context(), req.LicenseKey, fp)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrLicenseNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "license not found"})
		case errors.Is(err, service.ErrLicenseRevoked):
			c.JSON(http.StatusForbidden, gin.H{"error": "license revoked"})
		case errors.Is(err, service.ErrLicenseExpired):
			c.JSON(http.StatusForbidden, gin.H{"error": "license expired"})
		case errors.Is(err, service.ErrMaxActivations):
			c.JSON(http.StatusForbidden, gin.H{"error": "max activations reached"})
		case errors.Is(err, service.ErrHWIDMismatch):
			c.JSON(http.StatusForbidden, gin.H{"error": "hwid mismatch"})
		default:
			h.log.Error("activate", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}
	expStr := result.License.ExpiresAt.UTC().Format(time.RFC3339)
	nowStr := time.Now().UTC().Format(time.RFC3339)
	legacySig := license.SignResponse(h.secret, "status", "ok", "expires_at", expStr, "server_time", nowStr)
	c.JSON(http.StatusOK, ActivateResponse{
		Status:        "ok",
		ExpiresAt:     expStr,
		ServerTime:    nowStr,
		Signature:     legacySig,
		AccessToken:   result.AccessToken,
		RefreshToken:  result.RefreshToken,
		SignedLicense: result.SignedLicense,
		DeviceID:      result.Activation.DeviceID.String(),
	})
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
	DeviceID     string `json:"device_id" binding:"required"`
}

type RefreshResponse struct {
	AccessToken   string `json:"access_token"`
	RefreshToken  string `json:"refresh_token"`
	SignedLicense string `json:"signed_license"`
	ExpiresAt     string `json:"expires_at"`
	ServerTime    string `json:"server_time"`
}

func (h *LicenseHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	result, err := h.svc.Refresh(c.Request.Context(), req.RefreshToken, req.DeviceID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRefreshTokenInvalid):
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired refresh token"})
		case errors.Is(err, service.ErrLicenseRevoked):
			c.JSON(http.StatusForbidden, gin.H{"error": "license revoked"})
		case errors.Is(err, service.ErrLicenseExpired):
			c.JSON(http.StatusForbidden, gin.H{"error": "license expired"})
		default:
			h.log.Error("refresh", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}
	expStr := result.License.ExpiresAt.UTC().Format(time.RFC3339)
	nowStr := time.Now().UTC().Format(time.RFC3339)
	c.JSON(http.StatusOK, RefreshResponse{
		AccessToken:   result.AccessToken,
		RefreshToken:  result.RefreshToken,
		SignedLicense: result.SignedLicense,
		ExpiresAt:     expStr,
		ServerTime:    nowStr,
	})
}

type ValidateRequest struct {
	LicenseKey string `json:"license_key" binding:"required"`
	HWID       string `json:"hwid"`
}

func (h *LicenseHandler) Validate(c *gin.Context) {
	var req ValidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	status, _, err := h.svc.Validate(c.Request.Context(), req.LicenseKey, req.HWID)
	if err != nil {
		h.log.Error("validate", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": status})
}

type AdminRevokeRequest struct {
	LicenseKey string `json:"license_key" binding:"required"`
}

func (h *LicenseHandler) AdminRevoke(c *gin.Context) {
	var req AdminRevokeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := h.svc.Revoke(c.Request.Context(), req.LicenseKey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

type AdminActivateRequest struct {
	LicenseKey string `json:"license_key" binding:"required"`
	ExpiresAt  string `json:"expires_at" binding:"required"`
}

func (h *LicenseHandler) AdminActivate(c *gin.Context) {
	var req AdminActivateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	exp, err := time.Parse(time.RFC3339, req.ExpiresAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid expires_at"})
		return
	}
	if err := h.svc.AdminActivate(c.Request.Context(), req.LicenseKey, exp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

type AdminCreateRequest struct {
	LicenseKey     string `json:"license_key" binding:"required"`
	ExpiresAt      string `json:"expires_at" binding:"required"`
	MaxActivations int    `json:"max_activations"`
}

func (h *LicenseHandler) AdminCreate(c *gin.Context) {
	var req AdminCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	exp, err := time.Parse(time.RFC3339, req.ExpiresAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid expires_at (use RFC3339)"})
		return
	}
	max := req.MaxActivations
	if max <= 0 {
		max = 1
	}
	lic, err := h.svc.Create(c.Request.Context(), req.LicenseKey, exp, max)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"status":          "ok",
		"id":              lic.ID.String(),
		"license_key":     lic.LicenseKey,
		"expires_at":      lic.ExpiresAt.Format(time.RFC3339),
		"max_activations": lic.MaxActivations,
	})
}
