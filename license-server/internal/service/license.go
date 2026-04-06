package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"license-server/internal/license"
	"license-server/internal/models"
	"license-server/internal/refreshstore"
	"license-server/internal/repository"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	ErrLicenseNotFound    = errors.New("license not found")
	ErrLicenseRevoked    = errors.New("license revoked")
	ErrLicenseExpired    = errors.New("license expired")
	ErrMaxActivations    = errors.New("max activations reached")
	ErrHWIDMismatch      = errors.New("hwid mismatch")
	ErrRefreshTokenInvalid = errors.New("refresh token invalid")
)

// ActivateResult is returned on successful activate or refresh.
type ActivateResult struct {
	License       *models.License
	Activation    *models.Activation
	AccessToken   string
	RefreshToken  string
	SignedLicense string
}

type LicenseService struct {
	licRepo  *repository.LicenseRepository
	actRepo  *repository.ActivationRepository
	refresh  *redis.Client
	secret   string
	log      *zap.Logger
}

func NewLicenseService(licRepo *repository.LicenseRepository, actRepo *repository.ActivationRepository, refresh *redis.Client, secret string, log *zap.Logger) *LicenseService {
	return &LicenseService{licRepo: licRepo, actRepo: actRepo, refresh: refresh, secret: secret, log: log}
}

func hashFingerprint(fp string) string {
	h := sha256.Sum256([]byte(fp))
	return hex.EncodeToString(h[:])
}

func (s *LicenseService) Activate(ctx context.Context, key, deviceFingerprint string) (*ActivateResult, error) {
	if deviceFingerprint == "" {
		deviceFingerprint = "default"
	}
	fpHash := hashFingerprint(deviceFingerprint)

	lic, err := s.licRepo.FindByKey(ctx, key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrLicenseNotFound
		}
		return nil, fmt.Errorf("find: %w", err)
	}
	if lic.Status != "active" {
		return nil, ErrLicenseRevoked
	}
	if lic.ExpiresAt.Before(time.Now()) {
		return nil, ErrLicenseExpired
	}

	count, err := s.actRepo.CountByLicenseID(ctx, lic.ID)
	if err != nil {
		return nil, err
	}

	// Existing activation for this license + device?
	act, err := s.actRepo.FindByLicenseIDAndFingerprintHash(ctx, lic.ID, fpHash)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		// New activation
		if count >= int64(lic.MaxActivations) {
			return nil, ErrMaxActivations
		}
		act = &models.Activation{
			LicenseID:             lic.ID,
			DeviceID:              uuid.New(),
			DeviceFingerprintHash: fpHash,
		}
		if err := s.actRepo.Create(ctx, act); err != nil {
			return nil, err
		}
		// Update denormalized count on license for backward compat
		lic.CurrentActivations++
		_ = s.licRepo.Update(ctx, lic)
	} else {
		// Update last check
		now := time.Now()
		act.LastCheckAt = &now
		_ = s.actRepo.UpdateLastCheckAt(ctx, act)
	}

	// Reload license for ExpiresAt
	lic, _ = s.licRepo.FindByKey(ctx, key)
	return s.issueTokens(ctx, lic, act)
}

func (s *LicenseService) issueTokens(ctx context.Context, lic *models.License, act *models.Activation) (*ActivateResult, error) {
	now := time.Now().UTC()
	expStr := lic.ExpiresAt.UTC().Format(time.RFC3339)
	nowStr := now.Format(time.RFC3339)

	accessToken, err := license.NewAccessToken(s.secret, lic.ID, act.DeviceID)
	if err != nil {
		return nil, err
	}
	refreshToken := refreshstore.Token()
	if err := refreshstore.Store(ctx, s.refresh, refreshToken, lic.ID, act.DeviceID, 7*24*time.Hour); err != nil {
		return nil, err
	}

	payload := &license.SignedLicensePayload{
		LicenseID:  lic.ID.String(),
		DeviceID:   act.DeviceID.String(),
		ExpiresAt:  expStr,
		ServerTime: nowStr,
	}
	signedLicense, err := license.SignPayload(s.secret, payload)
	if err != nil {
		return nil, err
	}

	return &ActivateResult{
		License:       lic,
		Activation:    act,
		AccessToken:   accessToken,
		RefreshToken:  refreshToken,
		SignedLicense: signedLicense,
	}, nil
}

func (s *LicenseService) Refresh(ctx context.Context, refreshToken string, deviceIDStr string) (*ActivateResult, error) {
	entry, err := refreshstore.Get(ctx, s.refresh, refreshToken)
	if err != nil || entry == nil {
		return nil, ErrRefreshTokenInvalid
	}
	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		return nil, ErrRefreshTokenInvalid
	}
	if entry.DeviceID != deviceIDStr {
		return nil, ErrRefreshTokenInvalid
	}

	act, err := s.actRepo.FindByDeviceID(ctx, deviceID)
	if err != nil || act == nil {
		return nil, ErrRefreshTokenInvalid
	}
	lic, err := s.licRepo.FindByID(ctx, act.LicenseID)
	if err != nil || lic == nil {
		return nil, ErrRefreshTokenInvalid
	}
	if lic.Status != "active" {
		return nil, ErrLicenseRevoked
	}
	if lic.ExpiresAt.Before(time.Now()) {
		return nil, ErrLicenseExpired
	}

	// Delete old refresh token and issue new one (rotation)
	_ = refreshstore.Delete(ctx, s.refresh, refreshToken)
	now := time.Now()
	act.LastCheckAt = &now
	_ = s.actRepo.UpdateLastCheckAt(ctx, act)

	return s.issueTokens(ctx, lic, act)
}

func (s *LicenseService) ActivateLegacy(ctx context.Context, key, hwid string) (*models.License, error) {
	lic, err := s.licRepo.FindByKey(ctx, key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrLicenseNotFound
		}
		return nil, fmt.Errorf("find: %w", err)
	}
	if lic.Status != "active" {
		return nil, ErrLicenseRevoked
	}
	if lic.ExpiresAt.Before(time.Now()) {
		return nil, ErrLicenseExpired
	}
	if lic.CurrentActivations >= lic.MaxActivations {
		if lic.HWID == hwid {
			return lic, nil
		}
		return nil, ErrMaxActivations
	}
	if lic.HWID != "" && lic.HWID != hwid {
		return nil, ErrHWIDMismatch
	}
	if lic.HWID == "" {
		lic.HWID = hwid
	}
	lic.CurrentActivations++
	if err := s.licRepo.Update(ctx, lic); err != nil {
		return nil, err
	}
	s.log.Info("license activated (legacy)", zap.String("key", key), zap.String("hwid", hwid))
	return lic, nil
}

func (s *LicenseService) Validate(ctx context.Context, key, hwid string) (status string, lic *models.License, err error) {
	lic, err = s.licRepo.FindByKey(ctx, key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "invalid", nil, nil
		}
		return "", nil, err
	}
	if lic.Status == "revoked" {
		return "revoked", lic, nil
	}
	if lic.ExpiresAt.Before(time.Now()) {
		return "expired", lic, nil
	}
	if lic.HWID != "" && lic.HWID != hwid {
		return "invalid", lic, nil
	}
	return "active", lic, nil
}

func (s *LicenseService) Revoke(ctx context.Context, key string) error {
	lic, err := s.licRepo.FindByKey(ctx, key)
	if err != nil {
		return err
	}
	lic.Status = "revoked"
	if err := s.licRepo.Update(ctx, lic); err != nil {
		return err
	}
	if err := refreshstore.RevokeLicense(ctx, s.refresh, lic.ID); err != nil {
		s.log.Warn("revoke refresh tokens", zap.Error(err))
	}
	return nil
}

func (s *LicenseService) AdminActivate(ctx context.Context, key string, expiresAt time.Time) error {
	lic, err := s.licRepo.FindByKey(ctx, key)
	if err != nil {
		return err
	}
	lic.Status = "active"
	lic.ExpiresAt = expiresAt
	return s.licRepo.Update(ctx, lic)
}

// ListLicenses returns all licenses (admin dashboard).
func (s *LicenseService) ListLicenses(ctx context.Context) ([]models.License, error) {
	return s.licRepo.ListAll(ctx)
}

// AdminDelete removes a license and its activations; invalidates refresh tokens for that license.
func (s *LicenseService) AdminDelete(ctx context.Context, key string) error {
	lic, err := s.licRepo.FindByKey(ctx, key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrLicenseNotFound
		}
		return err
	}
	if err := refreshstore.RevokeLicense(ctx, s.refresh, lic.ID); err != nil {
		s.log.Warn("revoke refresh tokens on delete", zap.Error(err))
	}
	if err := s.actRepo.DeleteByLicenseID(ctx, lic.ID); err != nil {
		return err
	}
	if err := s.licRepo.DeleteByKey(ctx, key); err != nil {
		return err
	}
	s.log.Info("license deleted", zap.String("key", key))
	return nil
}

func (s *LicenseService) Create(ctx context.Context, licenseKey string, expiresAt time.Time, maxActivations int) (*models.License, error) {
	if maxActivations <= 0 {
		maxActivations = 1
	}
	lic := &models.License{
		LicenseKey:         licenseKey,
		Status:             "active",
		ExpiresAt:          expiresAt,
		MaxActivations:     maxActivations,
		CurrentActivations: 0,
	}
	if err := s.licRepo.Create(ctx, lic); err != nil {
		return nil, fmt.Errorf("create: %w", err)
	}
	s.log.Info("license created", zap.String("key", licenseKey), zap.Time("expires_at", expiresAt))
	return lic, nil
}
