package repository

import (
	"context"

	"license-server/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LicenseRepository struct {
	db *gorm.DB
}

func NewLicenseRepository(db *gorm.DB) *LicenseRepository {
	return &LicenseRepository{db: db}
}

func (r *LicenseRepository) FindByKey(ctx context.Context, key string) (*models.License, error) {
	var lic models.License
	err := r.db.WithContext(ctx).Where("license_key = ?", key).First(&lic).Error
	if err != nil {
		return nil, err
	}
	return &lic, nil
}

func (r *LicenseRepository) Update(ctx context.Context, lic *models.License) error {
	return r.db.WithContext(ctx).Save(lic).Error
}

func (r *LicenseRepository) Create(ctx context.Context, lic *models.License) error {
	return r.db.WithContext(ctx).Create(lic).Error
}

func (r *LicenseRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.License, error) {
	var lic models.License
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&lic).Error
	if err != nil {
		return nil, err
	}
	return &lic, nil
}

// ListAll returns all licenses ordered by expiration (soonest first).
func (r *LicenseRepository) ListAll(ctx context.Context) ([]models.License, error) {
	var list []models.License
	err := r.db.WithContext(ctx).Order("expires_at ASC").Find(&list).Error
	return list, err
}

// DeleteByKey permanently removes the license row (allows reusing the same license_key later).
func (r *LicenseRepository) DeleteByKey(ctx context.Context, key string) error {
	return r.db.WithContext(ctx).Unscoped().Where("license_key = ?", key).Delete(&models.License{}).Error
}
