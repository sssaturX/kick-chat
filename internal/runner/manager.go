package runner

import (
	"log"
	"sync"
)

const globalSendSlots = 20

// AccountManager управляет всеми AccountRunner и глобальным лимитом одновременных отправок.
type AccountManager struct {
	mu        sync.RWMutex
	runners   map[int]*AccountRunner
	doSend    SendFunc
	globalSem chan struct{}
}

// NewAccountManager создаёт менеджер с глобальным семафором на globalSendSlots отправок.
func NewAccountManager(sendFunc SendFunc) *AccountManager {
	return &AccountManager{
		runners:   make(map[int]*AccountRunner),
		doSend:    sendFunc,
		globalSem: make(chan struct{}, globalSendSlots),
	}
}

// SetSendFunc обновляет функцию отправки (для инициализации после создания).
func (m *AccountManager) SetSendFunc(f SendFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.doSend = f
}

// EnsureRunner создаёт runner для аккаунта, если его ещё нет. Вызывать при старте для каждого аккаунта.
func (m *AccountManager) EnsureRunner(accountID int) *AccountRunner {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.runners[accountID]; ok {
		return r
	}
	acquire := func() { m.globalSem <- struct{}{} }
	release := func() { <-m.globalSem }
	sendFunc := m.doSend
	if sendFunc == nil {
		sendFunc = func(int, SendTask) SendResult { return SendResult{} }
	}
	r := NewAccountRunner(accountID, sendFunc, acquire, release)
	m.runners[accountID] = r
	go r.Run()
	return r
}

// GetRunner возвращает runner по ID (nil если нет).
func (m *AccountManager) GetRunner(accountID int) *AccountRunner {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.runners[accountID]
}

// Send валидирует сообщение, находит runner и ставит задачу в очередь. Бизнес-логика здесь, не в HTTP.
// Возвращает (enqueued, reason).
func (m *AccountManager) Send(accountID int, message string) (ok bool, reason string) {
	return m.SendTask(accountID, SendTask{Message: message})
}

func (m *AccountManager) SendTask(accountID int, task SendTask) (ok bool, reason string) {
	r := m.GetRunner(accountID)
	if r == nil {
		r = m.EnsureRunner(accountID)
	}
	st := r.State()
	lastHash := st.LastMessageHash
	if task.AllowDuplicate || task.ReplyToMessageID != "" {
		lastHash = ""
	}
	ok, reason = ValidateMessage(task.Message, lastHash)
	if !ok {
		return false, reason
	}
	enqueued := r.Enqueue(task)
	if !enqueued {
		return false, "queue_full"
	}
	log.Printf("[manager] account %d enqueued message hash=%s reply_to=%t", accountID, HashMessage(task.Message), task.ReplyToMessageID != "")
	return true, ""
}

// RunnerStates возвращает состояние каждого runner (для дашборда и метрик).
func (m *AccountManager) RunnerStates() map[int]AccountState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[int]AccountState, len(m.runners))
	for id, r := range m.runners {
		out[id] = r.State()
	}
	return out
}

// Stop останавливает всех runners.
func (m *AccountManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.runners {
		r.Stop()
	}
	m.runners = make(map[int]*AccountRunner)
}
