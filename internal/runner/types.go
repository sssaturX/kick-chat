package runner

import "time"

// Состояния аккаунта (state machine).
const (
	StateOnline       = "ONLINE"
	StateRateLimited  = "RATE_LIMITED"
	StateCooldown     = "COOLDOWN"
	StateInvalid      = "INVALID"
	StateError        = "ERROR"
)

// SendTask — задача на отправку одного сообщения.
type SendTask struct {
	Message string
}

// SendResult — результат отправки (для backoff и смены состояния).
type SendResult struct {
	Err        error
	StatusCode int // 0 если неизвестно
}

// AccountState — текущее состояние аккаунта (для дашборда и метрик).
type AccountState struct {
	State           string    `json:"state"`
	CooldownUntil   time.Time `json:"cooldown_until"`
	BackoffUntil    time.Time `json:"backoff_until"`
	LastSendTime    time.Time `json:"last_send_time"`
	QueueSize       int       `json:"queue_size"`
	LastMessageHash string    `json:"last_message_hash"`
	SentTotal       int64     `json:"sent_total"`
	FailedTotal     int64     `json:"failed_total"`
}

// SendFunc выполняет одну отправку сообщения от имени аккаунта. Вызывается из worker.
type SendFunc func(accountID int, message string) SendResult
