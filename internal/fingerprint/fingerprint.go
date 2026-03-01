package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"runtime"
	"strings"
)

const salt = "kick-chat-license-v1"

// Device returns a stable device fingerprint string (SHA256 hex) for this machine.
// Uses hostname and machine-id where available so the same machine produces the same value.
func Device() string {
	var parts []string
	if h, err := os.Hostname(); err == nil && h != "" {
		parts = append(parts, h)
	}
	parts = append(parts, runtime.GOOS, runtime.GOARCH)
	// Linux: /etc/machine-id or /var/lib/dbus/machine-id
	if runtime.GOOS == "linux" {
		for _, p := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
			if b, err := os.ReadFile(p); err == nil && len(b) > 0 {
				parts = append(parts, strings.TrimSpace(string(b)))
				break
			}
		}
	}
	// Windows: COMPUTERNAME
	if runtime.GOOS == "windows" {
		if v := os.Getenv("COMPUTERNAME"); v != "" {
			parts = append(parts, v)
		}
	}
	// Fallback: use USER and a constant so we still get a stable value per user/host
	if u := os.Getenv("USER"); u != "" {
		parts = append(parts, u)
	}
	parts = append(parts, salt)
	h := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(h[:])
}
