package runner

import (
	"expvar"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// stringVar реализует expvar.Var без глобальной регистрации имени (избегаем "Reuse of exported var name").
type stringVar struct{ v atomic.Value }

func (s *stringVar) String() string {
	x := s.v.Load()
	if x == nil {
		return strconv.Quote("")
	}
	return strconv.Quote(x.(string))
}

var (
	messagesSentTotal   = expvar.NewInt("messages_sent_total")
	messagesFailedTotal = expvar.NewInt("messages_failed_total")
	avgSendLatencyNs    atomic.Uint64
	latencyCount        atomic.Uint64
	accountStateExpvar  = expvar.NewMap("account_state")
	accountStateVars    = make(map[int]*stringVar)
	accountStateMu      sync.Mutex
)

func init() {
	expvar.Publish("avg_send_latency_ns", expvar.Func(func() any {
		n := latencyCount.Load()
		if n == 0 {
			return int64(0)
		}
		return int64(avgSendLatencyNs.Load() / n)
	}))
}

// RecordSendSuccess учитывает успешную отправку и латентность (в наносекундах).
func RecordSendSuccess(latencyNs int64) {
	messagesSentTotal.Add(1)
	updateAvgLatency(latencyNs)
}

// RecordSendFailure учитывает неудачную отправку.
func RecordSendFailure() {
	messagesFailedTotal.Add(1)
}

func updateAvgLatency(latencyNs int64) {
	for {
		oldN := latencyCount.Load()
		oldSum := avgSendLatencyNs.Load()
		newN := oldN + 1
		newSum := oldSum + uint64(latencyNs)
		if latencyCount.CompareAndSwap(oldN, newN) && avgSendLatencyNs.CompareAndSwap(oldSum, newSum) {
			break
		}
	}
}

// PublishAccountStates обновляет expvar account_state для метрик (state по id).
func PublishAccountStates(states map[int]AccountState) {
	accountStateMu.Lock()
	defer accountStateMu.Unlock()
	accountStateExpvar.Init()
	for id, st := range states {
		v, ok := accountStateVars[id]
		if !ok {
			v = &stringVar{}
			v.v.Store(st.State)
			accountStateVars[id] = v
		} else {
			v.v.Store(st.State)
		}
		accountStateExpvar.Set(strconv.Itoa(id), v)
	}
}

// AvgSendLatency возвращает среднюю латентность (для логов).
func AvgSendLatency() time.Duration {
	n := latencyCount.Load()
	if n == 0 {
		return 0
	}
	return time.Duration(avgSendLatencyNs.Load()/n) * time.Nanosecond
}
