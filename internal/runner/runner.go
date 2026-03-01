package runner

import (
	"context"
	"log"
	"math/rand"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

const (
	// Без искусственной задержки: сообщения уходят сразу (rate.Inf). При 429/5xx по-прежнему cooldown/backoff.
	cooldownMin = 30 * time.Second
	cooldownMax = 90 * time.Second
	backoffInit = 5 * time.Second
	backoffMax  = 5 * time.Minute
)

// AccountRunner — один воркер на аккаунт: очередь, отправка без искусственной задержки (сразу).
type AccountRunner struct {
	ID            int
	Limiter       *rate.Limiter // rate.Inf — без задержки
	Queue         chan SendTask
	state         atomic.Value // *AccountState
	sendFunc      SendFunc
	acquireGlobal func()
	releaseGlobal func()
	stopCh        chan struct{}
}

// NewAccountRunner создаёт runner: сообщения отправляются сразу (без rate limit задержки).
func NewAccountRunner(id int, sendFunc SendFunc, acquireGlobal, releaseGlobal func()) *AccountRunner {
	r := &AccountRunner{
		ID:            id,
		Limiter:       rate.NewLimiter(rate.Inf, 1000), // без задержки
		Queue:         make(chan SendTask, 64),
		sendFunc:      sendFunc,
		acquireGlobal: acquireGlobal,
		releaseGlobal: releaseGlobal,
		stopCh:        make(chan struct{}),
	}
	st := &AccountState{State: StateOnline}
	r.state.Store(st)
	return r
}

// State возвращает копию текущего состояния (thread-safe).
func (r *AccountRunner) State() AccountState {
	v := r.state.Load()
	if v == nil {
		return AccountState{State: StateOnline}
	}
	st := *(v.(*AccountState))
	st.QueueSize = len(r.Queue)
	return st
}

func (r *AccountRunner) setState(st *AccountState) {
	r.state.Store(st)
	log.Printf("[runner %d] state=%s cooldown_until=%v backoff_until=%v", r.ID, st.State, st.CooldownUntil, st.BackoffUntil)
}

// Run запускает единственный worker: читает из Queue, захват глобального слота, отправка (без задержки).
func (r *AccountRunner) Run() {
	for {
		select {
		case <-r.stopCh:
			return
		case task, ok := <-r.Queue:
			if !ok {
				return
			}
			r.runOne(task)
		}
	}
}

func (r *AccountRunner) runOne(task SendTask) {
	st := r.State()
	if st.State == StateInvalid {
		return
	}
	// Ожидание cooldown/backoff
	now := time.Now()
	if now.Before(st.CooldownUntil) {
		time.Sleep(time.Until(st.CooldownUntil))
	}
	if now.Before(st.BackoffUntil) {
		time.Sleep(time.Until(st.BackoffUntil))
	}
	// Лимитер с rate.Inf не ждёт; оставлен для совместимости (cooldown/backoff выше).
	ctx, cancel := context.WithCancel(context.Background())
	go func() { <-r.stopCh; cancel() }()
	_ = r.Limiter.Wait(ctx)
	cancel()

	r.acquireGlobal()
	result := r.sendFunc(r.ID, task.Message)
	r.releaseGlobal()

	now = time.Now()
	next := r.applyResult(st, task.Message, result, now)
	r.setState(next)
}

func (r *AccountRunner) applyResult(prev AccountState, message string, res SendResult, now time.Time) *AccountState {
	next := &AccountState{
		LastSendTime:    now,
		QueueSize:       len(r.Queue),
		LastMessageHash: HashMessage(message),
		SentTotal:       prev.SentTotal,
		FailedTotal:     prev.FailedTotal,
	}
	if res.Err != nil {
		next.FailedTotal = prev.FailedTotal + 1
		next.SentTotal = prev.SentTotal
		switch {
		case res.StatusCode == 401:
			next.State = StateInvalid
			return next
		case res.StatusCode == 429:
			next.State = StateRateLimited
			next.CooldownUntil = now.Add(cooldownMin + time.Duration(rand.Int63n(int64(cooldownMax-cooldownMin))))
			return next
		case res.StatusCode >= 500:
			next.State = StateError
			backoff := backoffInit
			if !prev.BackoffUntil.IsZero() && prev.BackoffUntil.After(now) {
				backoff = time.Until(prev.BackoffUntil) * 2
			}
			if backoff > backoffMax {
				backoff = backoffMax
			}
			next.BackoffUntil = now.Add(backoff)
			return next
		default:
			next.State = StateError
			next.BackoffUntil = now.Add(backoffInit)
			return next
		}
	}
	next.SentTotal = prev.SentTotal + 1
	next.State = StateOnline
	next.CooldownUntil = time.Time{}
	next.BackoffUntil = time.Time{}
	return next
}

// Enqueue добавляет задачу в очередь. Не блокирует надолго; при переполнении очереди можно отклонить (здесь просто отправляем в chan).
func (r *AccountRunner) Enqueue(task SendTask) bool {
	st := r.State()
	if st.State == StateInvalid {
		return false
	}
	select {
	case r.Queue <- task:
		return true
	default:
		return false
	}
}

// Stop останавливает worker: закрытие очереди выходит из for, закрытие stopCh разблокирует Wait в runOne.
func (r *AccountRunner) Stop() {
	close(r.Queue)
	close(r.stopCh)
}
