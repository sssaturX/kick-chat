package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Activation struct {
	ID                     uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	LicenseID              uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:idx_license_fp" json:"license_id"`
	DeviceID               uuid.UUID  `gorm:"type:uuid;not null;index:idx_activation_device" json:"device_id"`
	DeviceFingerprintHash  string     `gorm:"type:varchar(64);not null;uniqueIndex:idx_license_fp" json:"-"` // SHA256 hex
	LastCheckAt            *time.Time `json:"last_check_at"`
	CreatedAt              time.Time  `json:"created_at"`

	License *License `gorm:"foreignKey:LicenseID" json:"-"`
}

func (Activation) TableName() string { return "activations" }

func (a *Activation) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	if a.DeviceID == uuid.Nil {
		a.DeviceID = uuid.New()
	}
	return nil
}
