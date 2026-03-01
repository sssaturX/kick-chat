package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type License struct {
	ID                 uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	LicenseKey         string         `gorm:"type:varchar(64);uniqueIndex;not null" json:"license_key"`
	Status             string         `gorm:"type:varchar(20);not null;default:active" json:"status"`
	ExpiresAt          time.Time      `gorm:"not null" json:"expires_at"`
	MaxActivations     int            `gorm:"default:1" json:"max_activations"`
	CurrentActivations int            `gorm:"default:0" json:"current_activations"`
	HWID               string         `gorm:"type:varchar(255)" json:"hwid"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
}

func (License) TableName() string { return "licenses" }

func (l *License) BeforeCreate(tx *gorm.DB) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return nil
}
