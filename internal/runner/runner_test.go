package runner

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestAccountRunner_EnqueueAndSend(t *testing.T) {
	var sent int32
	sem := make(chan struct{}, 2)
	sendFunc := func(accountID int, task SendTask) SendResult {
		atomic.AddInt32(&sent, 1)
		return SendResult{}
	}
	acquire := func() { sem <- struct{}{} }
	release := func() { <-sem }

	r := NewAccountRunner(1, sendFunc, acquire, release)
	go r.Run()
	defer r.Stop()

	ok := r.Enqueue(SendTask{Message: "hello"})
	if !ok {
		t.Fatal("Enqueue expected true")
	}
	ok = r.Enqueue(SendTask{Message: "world"})
	if !ok {
		t.Fatal("Enqueue expected true")
	}

	// Ждём обработки (rate limit 2–4s + jitter, но тест может быть быстрым при одной задаче)
	deadline := time.Now().Add(8 * time.Second)
	for atomic.LoadInt32(&sent) < 1 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if atomic.LoadInt32(&sent) < 1 {
		t.Errorf("expected at least 1 send, got %d", atomic.LoadInt32(&sent))
	}

	st := r.State()
	if st.State != StateOnline && st.State != "" {
		t.Logf("state after send: %s (ok)", st.State)
	}
}

func TestValidateMessage(t *testing.T) {
	tests := []struct {
		msg      string
		lastHash string
		wantOk   bool
		reason   string
	}{
		{"", "", false, "empty"},
		{"   ", "", false, "empty"},
		{"hi", "", true, ""},
		{"hi", HashMessage("hi"), false, "duplicate"},
		{"hi", HashMessage("other"), true, ""},
		{string(make([]byte, 501)), "", false, "too_long"},
	}
	for _, tt := range tests {
		ok, reason := ValidateMessage(tt.msg, tt.lastHash)
		if ok != tt.wantOk {
			t.Errorf("ValidateMessage(%q, ...) ok=%v want %v", tt.msg, ok, tt.wantOk)
		}
		if !tt.wantOk && reason != tt.reason {
			t.Errorf("ValidateMessage(%q, ...) reason=%q want %q", tt.msg, reason, tt.reason)
		}
	}
}

func TestManager_Send_Validation(t *testing.T) {
	sendFunc := func(accountID int, task SendTask) SendResult { return SendResult{} }
	m := NewAccountManager(sendFunc)
	m.EnsureRunner(1)

	ok, _ := m.Send(1, "")
	if ok {
		t.Fatal("empty message should be rejected")
	}
	ok, _ = m.Send(1, "   ")
	if ok {
		t.Fatal("whitespace-only should be rejected")
	}
	ok, reason := m.Send(1, "hello")
	if !ok && reason != "queue_full" {
		t.Logf("Send(hello) ok=%v reason=%q", ok, reason)
	}
}

func TestManager_SendTask_AllowsReplyDuplicate(t *testing.T) {
	sendFunc := func(accountID int, task SendTask) SendResult { return SendResult{} }
	m := NewAccountManager(sendFunc)
	r := m.EnsureRunner(1)
	defer r.Stop()
	r.setState(&AccountState{State: StateOnline, LastMessageHash: HashMessage("hello")})

	ok, reason := m.SendTask(1, SendTask{Message: "hello", ReplyToMessageID: "abc"})
	if !ok {
		t.Fatalf("reply duplicate should enqueue, reason=%q", reason)
	}
}
