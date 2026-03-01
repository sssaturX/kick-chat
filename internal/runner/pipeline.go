package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const maxMessageLen = 500

// ValidateMessage проверяет сообщение: длина, не пустое, не дубликат подряд.
// lastHash — хэш последнего отправленного сообщения с этого аккаунта.
// Возвращает (ok, reason).
func ValidateMessage(message string, lastHash string) (ok bool, reason string) {
	msg := strings.TrimSpace(message)
	if msg == "" {
		return false, "empty"
	}
	if len(msg) > maxMessageLen {
		return false, "too_long"
	}
	hash := hashMessage(msg)
	if hash != "" && hash == lastHash {
		return false, "duplicate"
	}
	return true, ""
}

func hashMessage(message string) string {
	h := sha256.Sum256([]byte(message))
	return hex.EncodeToString(h[:])
}

// HashMessage экспортирует хэш для логирования и метрик.
func HashMessage(message string) string {
	return hashMessage(strings.TrimSpace(message))
}
