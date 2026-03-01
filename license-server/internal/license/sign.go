package license

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Sign returns HMAC-SHA256 of message using secret, hex-encoded.
func Sign(secret, message string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

// Verify returns true if signature matches HMAC-SHA256(secret, message).
func Verify(secret, message, signature string) bool {
	return hmac.Equal([]byte(Sign(secret, message)), []byte(signature))
}

// SignResponse builds a canonical message from key-value pairs and signs it.
func SignResponse(secret string, pairs ...string) string {
	var msg string
	for i := 0; i+1 < len(pairs); i += 2 {
		if i > 0 {
			msg += "|"
		}
		msg += fmt.Sprintf("%s=%s", pairs[i], pairs[i+1])
	}
	return Sign(secret, msg)
}

// SignedLicensePayload is the canonical payload for offline verification in the binary.
type SignedLicensePayload struct {
	LicenseID  string `json:"license_id"`
	DeviceID   string `json:"device_id"`
	ExpiresAt  string `json:"expires_at"`
	ServerTime string `json:"server_time"`
}

// CanonicalMessage builds a deterministic string from the payload for signing.
func CanonicalMessage(p *SignedLicensePayload) string {
	// Sort keys for deterministic output
	m := map[string]string{
		"device_id":   p.DeviceID,
		"expires_at":  p.ExpiresAt,
		"license_id":  p.LicenseID,
		"server_time": p.ServerTime,
	}
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, m[k]))
	}
	return strings.Join(parts, "|")
}

// SignPayload signs the payload and returns "base64(payload_json).signature_hex".
func SignPayload(secret string, p *SignedLicensePayload) (string, error) {
	msg := CanonicalMessage(p)
	sig := Sign(secret, msg)
	raw, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s", hex.EncodeToString(raw), sig), nil
}

// VerifyPayload parses "payload_hex.signature_hex", verifies signature, and returns the payload.
func VerifyPayload(secret, signed string) (*SignedLicensePayload, error) {
	idx := strings.LastIndex(signed, ".")
	if idx <= 0 {
		return nil, fmt.Errorf("invalid signed license format")
	}
	payloadHex, sigHex := signed[:idx], signed[idx+1:]
	raw, err := hex.DecodeString(payloadHex)
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	var p SignedLicensePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}
	msg := CanonicalMessage(&p)
	if !Verify(secret, msg, sigHex) {
		return nil, fmt.Errorf("invalid signature")
	}
	return &p, nil
}
