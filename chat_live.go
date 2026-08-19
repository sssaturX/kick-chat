package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	kickPusherAppKey = "32cbd69e4b950bf97679"
	chatHistoryLimit = 200
)

type chatHub struct {
	ctx     context.Context
	slug    string
	mu      sync.Mutex
	clients map[chan []byte]struct{}
	history []chatMessage
	status  string
	seq     int64
}

type chatMessage struct {
	Seq       int64      `json:"seq"`
	ID        string     `json:"id"`
	UserID    int        `json:"user_id,omitempty"`
	Username  string     `json:"username"`
	Color     string     `json:"color,omitempty"`
	Content   string     `json:"content"`
	CreatedAt string     `json:"created_at"`
	ReplyTo   *chatReply `json:"reply_to,omitempty"`
}

type chatReply struct {
	ID       string `json:"id,omitempty"`
	Username string `json:"username,omitempty"`
	Content  string `json:"content,omitempty"`
}

func newChatHub(ctx context.Context, slug string) *chatHub {
	return &chatHub{
		ctx:     ctx,
		slug:    strings.TrimSpace(slug),
		clients: make(map[chan []byte]struct{}),
		status:  "connecting",
	}
}

func (h *chatHub) run() {
	if h == nil || h.slug == "" {
		return
	}
	for {
		select {
		case <-h.ctx.Done():
			return
		default:
		}
		if err := h.runOnce(); err != nil {
			h.setStatus("reconnecting: " + err.Error())
			log.Printf("[chat] %v", err)
		}
		select {
		case <-h.ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (h *chatHub) runOnce() error {
	chatroomID, err := h.resolveChatroomID()
	if err != nil {
		return err
	}
	h.setStatus("connecting")
	u := url.URL{
		Scheme:   "wss",
		Host:     "ws-us2.pusher.com",
		Path:     "/app/" + kickPusherAppKey,
		RawQuery: "protocol=7&client=js&version=8.4.0&flash=false",
	}
	header := http.Header{}
	header.Set("Origin", "https://kick.com")
	header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	conn, _, err := (&websocket.Dialer{HandshakeTimeout: 15 * time.Second}).DialContext(h.ctx, u.String(), header)
	if err != nil {
		return fmt.Errorf("chat websocket: %w", err)
	}
	defer conn.Close()

	channel := fmt.Sprintf("chatrooms.%d.v2", chatroomID)
	if err := conn.WriteJSON(map[string]interface{}{
		"event": "pusher:subscribe",
		"data":  map[string]string{"channel": channel},
	}); err != nil {
		return fmt.Errorf("chat subscribe: %w", err)
	}
	h.setStatus("connected")
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("chat read: %w", err)
		}
		var env struct {
			Event string          `json:"event"`
			Data  json.RawMessage `json:"data"`
		}
		if json.Unmarshal(payload, &env) != nil {
			continue
		}
		switch env.Event {
		case "pusher:ping":
			_ = conn.WriteJSON(map[string]string{"event": "pusher:pong", "data": "{}"})
		case "pusher_internal:subscription_succeeded":
			h.setStatus("connected")
		default:
			if !strings.Contains(env.Event, "ChatMessageEvent") {
				continue
			}
			msg, ok := parseKickChatMessage(rawPusherData(env.Data))
			if !ok {
				continue
			}
			msg.Seq = atomic.AddInt64(&h.seq, 1)
			if msg.CreatedAt == "" {
				msg.CreatedAt = time.Now().UTC().Format(time.RFC3339)
			}
			h.addMessage(msg)
		}
	}
}

func rawPusherData(raw json.RawMessage) []byte {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return []byte(s)
	}
	return raw
}

func (h *chatHub) resolveChatroomID() (int64, error) {
	endpoint := "https://kick.com/api/v2/channels/" + url.PathEscape(strings.ToLower(h.slug))
	req, _ := http.NewRequestWithContext(h.ctx, http.MethodGet, endpoint, nil)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Referer", "https://kick.com/"+h.slug)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return 0, fmt.Errorf("chatroom: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("chatroom status %d", resp.StatusCode)
	}
	var data struct {
		Chatroom struct {
			ID int64 `json:"id"`
		} `json:"chatroom"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, fmt.Errorf("chatroom decode: %w", err)
	}
	if data.Chatroom.ID == 0 {
		return 0, fmt.Errorf("chatroom id not found")
	}
	return data.Chatroom.ID, nil
}

func (h *chatHub) addMessage(msg chatMessage) {
	data, _ := json.Marshal(msg)
	h.mu.Lock()
	h.history = append(h.history, msg)
	if len(h.history) > chatHistoryLimit {
		h.history = h.history[len(h.history)-chatHistoryLimit:]
	}
	for ch := range h.clients {
		select {
		case ch <- data:
		default:
		}
	}
	h.mu.Unlock()
}

func (h *chatHub) setStatus(status string) {
	h.mu.Lock()
	h.status = status
	h.mu.Unlock()
}

func (h *chatHub) snapshot() ([]chatMessage, string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]chatMessage, len(h.history))
	copy(out, h.history)
	return out, h.status
}

func (h *chatHub) register() chan []byte {
	ch := make(chan []byte, 64)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *chatHub) unregister(ch chan []byte) {
	h.mu.Lock()
	delete(h.clients, ch)
	close(ch)
	h.mu.Unlock()
}

func parseKickChatMessage(data []byte) (chatMessage, bool) {
	var root map[string]interface{}
	if json.Unmarshal(data, &root) != nil {
		return chatMessage{}, false
	}
	m := objectMap(root)
	if nested := asMap(m["message"]); nested != nil {
		m = nested
	}
	content := stringValue(m, "content", "message")
	if strings.TrimSpace(content) == "" {
		return chatMessage{}, false
	}
	sender := asMap(firstValue(m, "sender", "user"))
	msg := chatMessage{
		ID:        stringValue(m, "id", "message_id"),
		Content:   content,
		CreatedAt: stringValue(m, "created_at", "createdAt", "sent_at"),
		Username:  stringValue(sender, "username", "name"),
		Color:     senderColor(sender),
		UserID:    intValue(sender, "id", "user_id"),
	}
	if msg.ID == "" {
		msg.ID = stringValue(root, "id", "message_id")
	}
	if msg.Username == "" {
		msg.Username = "chat"
	}
	if reply := asMap(firstValue(m, "replies_to", "reply_to", "reply")); reply != nil {
		replySender := asMap(firstValue(reply, "sender", "user"))
		msg.ReplyTo = &chatReply{
			ID:       stringValue(reply, "id", "message_id"),
			Username: stringValue(replySender, "username", "name"),
			Content:  stringValue(reply, "content", "message"),
		}
	}
	return msg, true
}

func objectMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return map[string]interface{}{}
	}
	return m
}

func firstValue(m map[string]interface{}, keys ...string) interface{} {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			return v
		}
	}
	return nil
}

func asMap(v interface{}) map[string]interface{} {
	m, _ := v.(map[string]interface{})
	return m
}

func stringValue(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		v, ok := m[key]
		if !ok || v == nil {
			continue
		}
		switch x := v.(type) {
		case string:
			return x
		case float64:
			return strconv.FormatInt(int64(x), 10)
		case json.Number:
			return x.String()
		}
	}
	return ""
}

func intValue(m map[string]interface{}, keys ...string) int {
	for _, key := range keys {
		v, ok := m[key]
		if !ok || v == nil {
			continue
		}
		switch x := v.(type) {
		case float64:
			return int(x)
		case int:
			return x
		case string:
			n, _ := strconv.Atoi(x)
			return n
		}
	}
	return 0
}

func senderColor(sender map[string]interface{}) string {
	if sender == nil {
		return ""
	}
	if color := stringValue(sender, "username_color", "color"); color != "" {
		return color
	}
	if identity := asMap(sender["identity"]); identity != nil {
		return stringValue(identity, "username_color", "color")
	}
	return ""
}
