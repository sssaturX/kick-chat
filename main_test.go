package main

import (
	"context"
	"testing"
	"time"

	"kick-chat-go/internal/runner"

	"github.com/henrikah/kick-go-sdk/v2/kickapitypes"
)

func TestPopShuffledMessageUsesEveryLineBeforeRepeat(t *testing.T) {
	messages := []string{"a", "b", "c", "d"}
	seen := make(map[string]int)
	var deck []string
	for i := 0; i < len(messages); i++ {
		msg, ok := popShuffledMessage(&deck, messages)
		if !ok {
			t.Fatalf("expected message at step %d", i+1)
		}
		seen[msg]++
	}
	if len(seen) != len(messages) {
		t.Fatalf("expected %d unique messages in cycle, got %d: %v", len(messages), len(seen), seen)
	}
	for _, msg := range messages {
		if seen[msg] != 1 {
			t.Fatalf("message %q sent %d times in one cycle", msg, seen[msg])
		}
	}
	msg, ok := popShuffledMessage(&deck, messages)
	if !ok || msg == "" {
		t.Fatal("expected next cycle to start after deck exhausted")
	}
}

func TestNormalizePresetMessagesRemovesDuplicates(t *testing.T) {
	got, err := normalizePresetMessages([]string{" left ", "right", "left", ""})
	if err != nil {
		t.Fatalf("normalizePresetMessages: %v", err)
	}
	if len(got) != 2 || got[0] != "left" || got[1] != "right" {
		t.Fatalf("normalized messages = %v", got)
	}
}

func TestStartAutosendSwitchesActivePreset(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := newAccountsStore("")
	store.Accounts = []account{{ID: 11, Token: "token"}}
	manager := runner.NewAccountManager(nil)
	defer manager.Stop()
	srv := &webServer{ctx: ctx, store: store, manager: manager, adminRelease: true}

	if err := srv.startAutosend([]int{1}, []string{"first"}, "first preset", 86400, 86400); err != nil {
		t.Fatalf("start first preset: %v", err)
	}
	if err := srv.startAutosend([]int{1}, []string{"second"}, "second preset", 86400, 86400); err != nil {
		t.Fatalf("switch preset: %v", err)
	}
	srv.autosendMu.Lock()
	active := srv.autosendPresetName
	messages := append([]string(nil), srv.autosendMessages...)
	srv.autosendMu.Unlock()
	if active != "second preset" || len(messages) != 1 || messages[0] != "second" {
		t.Fatalf("active preset did not switch: active=%q messages=%v", active, messages)
	}
	srv.stopAutosend()
}

func TestStreamStatusFromChannel(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	channel := kickapitypes.ChannelData{
		StreamTitle: "Test stream",
		Stream: kickapitypes.ChannelStream{
			IsLive:      true,
			StartTime:   "2026-08-12T10:30:00Z",
			ViewerCount: 321,
		},
	}
	status := streamStatusFromChannel("streamer", channel, now)
	if !status.IsLive || status.UptimeSeconds != 90*60 || status.ViewerCount != 321 {
		t.Fatalf("unexpected live status: %+v", status)
	}
	channel.Stream = kickapitypes.ChannelStream{}
	offline := streamStatusFromChannel("streamer", channel, now)
	if offline.IsLive || offline.UptimeSeconds != 0 {
		t.Fatalf("unexpected offline status: %+v", offline)
	}
}

func TestNormalizeKickEmojiContentUsesKickNativeEmoteToken(t *testing.T) {
	want := "[emote:37226:KEKW]"
	if got := normalizeKickEmojiContent(":KEKW:"); got != want {
		t.Fatalf("normalizeKickEmojiContent(:KEKW:) = %q", got)
	}
	if got := normalizeKickEmojiContent("KEKW"); got != want {
		t.Fatalf("normalizeKickEmojiContent(KEKW) = %q", got)
	}
	if got := normalizeKickEmojiContent(want); got != want {
		t.Fatalf("normalizeKickEmojiContent(native token) = %q", got)
	}
	if got := normalizeKickEmojiContent("hello KEKW"); got != "hello KEKW" {
		t.Fatalf("mixed message changed unexpectedly: %q", got)
	}
	if !isKnownKickEmojiContent(want) {
		t.Fatal("native emote token should be known")
	}
}

func TestNormalizeChannelSlugInput(t *testing.T) {
	cases := map[string]string{
		"examplechannel":                         "examplechannel",
		"@examplechannel":                        "examplechannel",
		"https://kick.com/examplechannel?chat=1": "examplechannel",
	}
	for input, want := range cases {
		got, err := normalizeChannelSlugInput(input)
		if err != nil {
			t.Fatalf("normalizeChannelSlugInput(%q) error: %v", input, err)
		}
		if got != want {
			t.Fatalf("normalizeChannelSlugInput(%q) = %q, want %q", input, got, want)
		}
	}
	if _, err := normalizeChannelSlugInput("bad channel"); err == nil {
		t.Fatal("expected invalid slug error")
	}
}
