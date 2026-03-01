package license

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const accessTokenExpiry = 15 * time.Minute

// AccessClaims for JWT access token.
type AccessClaims struct {
	jwt.RegisteredClaims
	LicenseID string `json:"license_id"`
	DeviceID  string `json:"device_id"`
}

// NewAccessToken creates a JWT signed with HMAC (HS256) for the given license and device.
func NewAccessToken(secret string, licenseID, deviceID uuid.UUID) (string, error) {
	now := time.Now().UTC()
	claims := AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.New().String(),
		},
		LicenseID: licenseID.String(),
		DeviceID:  deviceID.String(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString([]byte(secret))
}

// ParseAccessToken verifies and parses the JWT.
func ParseAccessToken(secret, tokenString string) (*AccessClaims, error) {
	tok, err := jwt.ParseWithClaims(tokenString, &AccessClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := tok.Claims.(*AccessClaims)
	if !ok || !tok.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}
