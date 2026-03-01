package licensestore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

const (
	keySalt     = "kick-license-store-v1"
	iterations  = 100000
	keyLen      = 32
	nonceSize   = 12
)

// Payload is the decrypted license data stored on disk.
type Payload struct {
	SignedLicense    string    `json:"signed_license"`
	RefreshToken     string    `json:"refresh_token"`
	DeviceID         string    `json:"device_id"`
	LastValidationAt time.Time `json:"last_validation_at"`
}

// Store reads/writes encrypted license payload. Key is derived from machine fingerprint + salt.
type Store struct {
	filePath string
	fingerprint string
}

func NewStore(filePath string, deviceFingerprint string) *Store {
	if filePath == "" {
		filePath = ".kick_license.dat"
	}
	return &Store{filePath: filePath, fingerprint: deviceFingerprint}
}

func (s *Store) key() ([]byte, error) {
	key := pbkdf2.Key([]byte(s.fingerprint), []byte(keySalt), iterations, keyLen, sha256.New)
	return key, nil
}

func (s *Store) Load() (*Payload, error) {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(data) < nonceSize {
		return nil, fmt.Errorf("license file too short")
	}
	k, err := s.key()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt license: %w", err)
	}
	var p Payload
	if err := json.Unmarshal(plain, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) Save(p *Payload) error {
	plain, err := json.Marshal(p)
	if err != nil {
		return err
	}
	k, err := s.key()
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(k)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	ciphertext := gcm.Seal(nil, nonce, plain, nil)
	out := append(nonce, ciphertext...)
	return os.WriteFile(s.filePath, out, 0600)
}

func (s *Store) Delete() error {
	err := os.Remove(s.filePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// VerifySignedLicense verifies HMAC of the signed_license payload and checks expiry and device match.
// signed format: "hex(payload_json).signature_hex". secret is LICENSE_HMAC_SECRET (same as server).
func VerifySignedLicense(secret, signed, deviceID string) (expiresAt time.Time, err error) {
	if secret == "" || signed == "" {
		return time.Time{}, fmt.Errorf("missing secret or signed license")
	}
	idx := strings.LastIndex(signed, ".")
	if idx <= 0 {
		return time.Time{}, fmt.Errorf("invalid signed license format")
	}
	payloadHex, sigHex := signed[:idx], signed[idx+1:]
	raw, err := hex.DecodeString(payloadHex)
	if err != nil {
		return time.Time{}, fmt.Errorf("decode payload: %w", err)
	}
	var p struct {
		LicenseID  string `json:"license_id"`
		DeviceID   string `json:"device_id"`
		ExpiresAt  string `json:"expires_at"`
		ServerTime string `json:"server_time"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return time.Time{}, err
	}
	// Canonical message (same order as license-server)
	msg := "device_id=" + p.DeviceID + "|expires_at=" + p.ExpiresAt + "|license_id=" + p.LicenseID + "|server_time=" + p.ServerTime
	if !verifyHMAC(secret, msg, sigHex) {
		return time.Time{}, fmt.Errorf("invalid signature")
	}
	if p.DeviceID != deviceID {
		return time.Time{}, fmt.Errorf("device_id mismatch")
	}
	expiresAt, err = time.Parse(time.RFC3339, p.ExpiresAt)
	if err != nil {
		return time.Time{}, err
	}
	if expiresAt.Before(time.Now()) {
		return time.Time{}, fmt.Errorf("license expired")
	}
	return expiresAt, nil
}

func verifyHMAC(secret, message, signature string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}
