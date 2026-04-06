package repository

import (
	"context"

	"license-server/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ActivationRepository struct {
	db *gorm.DB
}

func NewActivationRepository(db *gorm.DB) *ActivationRepository {
	return &ActivationRepository{db: db}
}

func (r *ActivationRepository) Create(ctx context.Context, a *models.Activation) error {
	return r.db.WithContext(ctx).Create(a).Error
}

func (r *ActivationRepository) FindByLicenseIDAndFingerprintHash(ctx context.Context, licenseID uuid.UUID, fingerprintHash string) (*models.Activation, error) {
	var a models.Activation
	err := r.db.WithContext(ctx).Where("license_id = ? AND device_fingerprint_hash = ?", licenseID, fingerprintHash).First(&a).Error
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *ActivationRepository) FindByDeviceID(ctx context.Context, deviceID uuid.UUID) (*models.Activation, error) {
	var a models.Activation
	err := r.db.WithContext(ctx).Where("device_id = ?", deviceID).Preload("License").First(&a).Error
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *ActivationRepository) CountByLicenseID(ctx context.Context, licenseID uuid.UUID) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.Activation{}).Where("license_id = ?", licenseID).Count(&n).Error
	return n, err
}

func (r *ActivationRepository) UpdateLastCheckAt(ctx context.Context, a *models.Activation) error {
	return r.db.WithContext(ctx).Model(a).Update("last_check_at", a.LastCheckAt).Error
}

func (r *ActivationRepository) DeleteByLicenseID(ctx context.Context, licenseID uuid.UUID) error {
	return r.db.WithContext(ctx).Unscoped().Where("license_id = ?", licenseID).Delete(&models.Activation{}).Error
}
