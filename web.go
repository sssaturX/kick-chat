package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"kick-chat-go/internal/licensestore"
	"kick-chat-go/internal/runner"
	"kick-chat-go/internal/viewerbot"

	"github.com/henrikah/kick-go-sdk/v2"
	"github.com/henrikah/kick-go-sdk/v2/enums/kickscopes"
	"github.com/henrikah/kick-go-sdk/v2/kickapitypes"
	"github.com/henrikah/kick-go-sdk/v2/kickcontracts"
)

//go:embed static
var staticFS embed.FS

// licenseCheckClient — таймаут для запросов к серверу лицензий, чтобы не блокировать загрузку дашборда.
var licenseCheckClient = &http.Client{Timeout: 8 * time.Second}

// licenseGate guards dashboard and API when license server is configured.
type licenseGate struct {
	mu    sync.RWMutex
	valid bool
}

type streamStatus struct {
	Slug          string    `json:"slug"`
	IsLive        bool      `json:"is_live"`
	StartTime     string    `json:"start_time,omitempty"`
	ViewerCount   int       `json:"viewer_count"`
	Title         string    `json:"title,omitempty"`
	UptimeSeconds int64     `json:"uptime_seconds"`
	CheckedAt     time.Time `json:"checked_at"`
}

func (g *licenseGate) Valid() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.valid
}

func (g *licenseGate) SetValid(v bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.valid = v
}

type webServer struct {
	ctx               context.Context
	store             *accountsStore
	channelSlug       string
	channelMu         sync.RWMutex
	broadcasterID     int
	broadcasterMu     sync.Mutex
	chatHub           *chatHub
	chatCancel        context.CancelFunc
	manager           *runner.AccountManager
	viewerBot         *viewerbot.Bot
	onChannelChanged  func(string)
	onShutdown        func()
	oauthClient       kickcontracts.OAuthClient
	clientID          string
	clientSecret      string
	oauthRedirectURI  string // Initiate + exchange — must match Redirect URL in Kick app
	dashboardBaseURL  string // e.g. http://localhost:8080 — redirects after OAuth land here
	licenseGate       *licenseGate
	licenseStore      *licensestore.Store
	licenseServerURL  string
	licenseHMACSecret string
	deviceFingerprint string
	// skipLicense: SKIP_LICENSE=1 — локальный тест без license-server и ключа
	skipLicense bool
	// adminRelease: scripts/build-release-admin.* — без лицензии, можно слать одинаковые сообщения подряд
	adminRelease bool

	// autosend: sends prepared messages from auto-sender.txt by selected accounts.
	autosendMu          sync.Mutex
	autosendEnabled     bool
	autosendMinSec      int
	autosendMaxSec      int
	autosendAccountIDs  []int
	autosendNextIndex   int
	autosendShuffleDeck []string // admin: random order without repeats until file cycle ends
	autosendMessages    []string // non-empty for an active admin event preset
	autosendPresetName  string
	autosendStopCh      chan struct{}

	streamStatusMu       sync.Mutex
	streamStatusCached   streamStatus
	streamStatusCachedAt time.Time
}

func (srv *webServer) currentChannelSlug() string {
	srv.channelMu.RLock()
	defer srv.channelMu.RUnlock()
	return srv.channelSlug
}

func (srv *webServer) currentChatHub() *chatHub {
	srv.channelMu.RLock()
	defer srv.channelMu.RUnlock()
	return srv.chatHub
}

func (srv *webServer) switchChannel(slug string) {
	srv.channelMu.Lock()
	if srv.chatCancel != nil {
		srv.chatCancel()
		srv.chatCancel = nil
	}
	srv.channelSlug = slug
	srv.chatHub = nil
	if slug != "" {
		chatCtx, cancel := context.WithCancel(srv.ctx)
		hub := newChatHub(chatCtx, slug)
		srv.chatHub = hub
		srv.chatCancel = cancel
		go hub.run()
	}
	srv.channelMu.Unlock()

	srv.broadcasterMu.Lock()
	srv.broadcasterID = 0
	srv.broadcasterMu.Unlock()

	srv.streamStatusMu.Lock()
	srv.streamStatusCached = streamStatus{}
	srv.streamStatusCachedAt = time.Time{}
	srv.streamStatusMu.Unlock()

	if srv.onChannelChanged != nil {
		srv.onChannelChanged(slug)
	}
}

func (w *webServer) resolveBroadcasterID(token, proxy string) int {
	slug := w.currentChannelSlug()
	if slug == "" {
		return 0
	}
	client := apiClientWithProxy(proxy)
	api, err := kick.NewAPIClient(kickapitypes.APIClientConfig{HTTPClient: client})
	if err != nil {
		return 0
	}
	channels, err := api.Channel().GetChannelsByBroadcasterSlug(w.ctx, token, []string{slug})
	if err != nil && proxy != "" && isNetworkOrProxyError(err) {
		api, _ = kick.NewAPIClient(kickapitypes.APIClientConfig{HTTPClient: http.DefaultClient})
		channels, err = api.Channel().GetChannelsByBroadcasterSlug(w.ctx, token, []string{slug})
	}
	if err != nil || len(channels.Data) == 0 {
		return 0
	}
	return channels.Data[0].BroadcasterUserID
}

func (w *webServer) getBroadcasterID() int {
	w.broadcasterMu.Lock()
	defer w.broadcasterMu.Unlock()
	if w.broadcasterID != 0 {
		return w.broadcasterID
	}
	for i := 1; i <= w.store.Count(); i++ {
		tok, _, proxy, _, ok := w.store.GetAccountByIndex(i)
		if !ok {
			continue
		}
		if id := w.resolveBroadcasterID(tok, proxy); id != 0 {
			w.broadcasterID = id
			return id
		}
	}
	return 0
}

func (srv *webServer) handleAPIAccounts(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	type acc struct {
		ID       int    `json:"id"`
		StableID int    `json:"stable_id"`
		Name     string `json:"name"` // custom label; empty = show "Account N" in UI
		Current  bool   `json:"current"`
		Proxy    string `json:"proxy"`
	}
	var list []acc
	for _, a := range srv.store.List() {
		_, _, proxy, _, _ := srv.store.GetAccountByIndex(a.Num)
		list = append(list, acc{
			ID:       a.Num,
			StableID: a.StableID,
			Name:     srv.store.NameAt(a.Num),
			Current:  a.Current,
			Proxy:    proxy,
		})
	}
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(list)
}

func (srv *webServer) handleAPIAccountsOrder(rw http.ResponseWriter, r *http.Request) {
	if !srv.adminRelease {
		http.Error(rw, "admin release only", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPut {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Order []int `json:"order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(rw, "invalid json", http.StatusBadRequest)
		return
	}
	if err := srv.store.SetDisplayOrder(body.Order); err != nil {
		http.Error(rw, "order must contain every account exactly once", http.StatusBadRequest)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(map[string]bool{"ok": true})
}

func (srv *webServer) handleAPIStatus(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	type st struct {
		ID     int  `json:"id"`
		Online bool `json:"online"`
	}
	states := srv.manager.RunnerStates()
	var list []st
	for i := 1; i <= srv.store.Count(); i++ {
		state := runner.StateOnline
		if current, ok := states[i]; ok && strings.TrimSpace(current.State) != "" {
			state = current.State
		}
		list = append(list, st{ID: i, Online: state != runner.StateInvalid && state != runner.StateError})
	}
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(list)
}

func (srv *webServer) handleAPISend(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		AccountID        int    `json:"account_id"`
		Message          string `json:"message"`
		ReplyToMessageID string `json:"reply_to_message_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(rw, "invalid json", http.StatusBadRequest)
		return
	}
	if body.AccountID < 1 {
		http.Error(rw, "account_id required", http.StatusBadRequest)
		return
	}
	task := srv.sendTaskForMessage(body.Message, body.ReplyToMessageID)
	ok, reason := srv.manager.SendTask(body.AccountID, task)
	if !ok {
		http.Error(rw, reason, http.StatusBadRequest)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(map[string]string{"status": "ok"})
}

func (srv *webServer) handleAPIChatHistory(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	hub := srv.currentChatHub()
	if hub == nil {
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]interface{}{
			"status":   "disabled",
			"messages": []chatMessage{},
		})
		return
	}
	messages, status := hub.snapshot()
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(map[string]interface{}{
		"status":   status,
		"messages": messages,
	})
}

func writeSSEJSON(rw http.ResponseWriter, event string, payload interface{}) bool {
	data, err := json.Marshal(payload)
	if err != nil {
		return false
	}
	if event != "" {
		if _, err := fmt.Fprintf(rw, "event: %s\n", event); err != nil {
			return false
		}
	}
	_, err = fmt.Fprintf(rw, "data: %s\n\n", data)
	return err == nil
}

func (srv *webServer) handleAPIChatEvents(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	hub := srv.currentChatHub()
	if hub == nil {
		http.Error(rw, "chat disabled", http.StatusServiceUnavailable)
		return
	}
	flusher, ok := rw.(http.Flusher)
	if !ok {
		http.Error(rw, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	rw.Header().Set("Content-Type", "text/event-stream")
	rw.Header().Set("Cache-Control", "no-store")
	rw.Header().Set("Connection", "keep-alive")
	rw.Header().Set("X-Accel-Buffering", "no")

	ch := hub.register()
	defer hub.unregister(ch)

	_, status := hub.snapshot()
	lastStatus := status
	if writeSSEJSON(rw, "status", map[string]string{"status": status}) {
		flusher.Flush()
	}

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case data := <-ch:
			if _, err := fmt.Fprintf(rw, "data: %s\n\n", data); err != nil {
				return
			}
			flusher.Flush()
		case <-ticker.C:
			_, status := hub.snapshot()
			if status != lastStatus {
				lastStatus = status
				if !writeSSEJSON(rw, "status", map[string]string{"status": status}) {
					return
				}
			} else if _, err := fmt.Fprint(rw, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		case <-hub.ctx.Done():
			return
		case <-srv.ctx.Done():
			return
		}
	}
}

const (
	messagesFileName          = "messages.txt"
	autoSenderFileName        = "auto-sender.txt"
	autoSenderPresetsFileName = "auto-sender-presets.json"
)

type autoSenderPreset struct {
	Name             string    `json:"name"`
	Messages         []string  `json:"messages,omitempty"`
	AccountStableIDs []int     `json:"account_stable_ids,omitempty"`
	MinSec           int       `json:"min_sec"`
	MaxSec           int       `json:"max_sec"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type autoSenderPresetsFile struct {
	Active  string             `json:"active"`
	Presets []autoSenderPreset `json:"presets"`
}

func readNonEmptyLinesFile(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	out := make([]string, 0, len(lines))
	for _, s := range lines {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// readMessagesFile returns lines from messages.txt, one non-empty line per preset.
func readMessagesFile() []string {
	return readNonEmptyLinesFile(messagesFileName)
}

func readAutoSenderMessages() []string {
	return readNonEmptyLinesFile(autoSenderFileName)
}

func normalizePresetMessages(messages []string) ([]string, error) {
	seen := make(map[string]bool, len(messages))
	out := make([]string, 0, len(messages))
	for _, message := range messages {
		message = strings.TrimSpace(message)
		if message == "" || seen[message] {
			continue
		}
		if len(message) > 500 {
			return nil, fmt.Errorf("preset message is longer than 500 characters")
		}
		seen[message] = true
		out = append(out, message)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("add at least one message")
	}
	return out, nil
}

func normalizePresetName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("preset name required")
	}
	if len(name) > 64 {
		return "", fmt.Errorf("preset name is too long")
	}
	return name, nil
}

func readAutoSenderPresetsFile() (autoSenderPresetsFile, error) {
	data, err := os.ReadFile(autoSenderPresetsFileName)
	if err != nil {
		if os.IsNotExist(err) {
			return autoSenderPresetsFile{}, nil
		}
		return autoSenderPresetsFile{}, err
	}
	var file autoSenderPresetsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return autoSenderPresetsFile{}, err
	}
	for i := range file.Presets {
		file.Presets[i].MinSec, file.Presets[i].MaxSec = sanitizeAutosendRange(file.Presets[i].MinSec, file.Presets[i].MaxSec)
	}
	return file, nil
}

func writeAutoSenderPresetsFile(file autoSenderPresetsFile) error {
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(autoSenderPresetsFileName, data, 0644)
}

func findAutoSenderPreset(file autoSenderPresetsFile, name string) (autoSenderPreset, int, bool) {
	for i, preset := range file.Presets {
		if strings.EqualFold(strings.TrimSpace(preset.Name), strings.TrimSpace(name)) {
			return preset, i, true
		}
	}
	return autoSenderPreset{}, -1, false
}

func (srv *webServer) handleAPIMessages(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	messages := readMessagesFile()
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(map[string]interface{}{"messages": messages})
}

func (srv *webServer) handleAPIEmotes(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(map[string]interface{}{"emotes": kickEmojiDefinitions()})
}

func sanitizeAutosendRange(minSec, maxSec int) (int, int) {
	if minSec <= 0 {
		minSec = 60
	}
	if minSec < 1 {
		minSec = 1
	}
	if minSec > 86400 {
		minSec = 86400
	}
	if maxSec <= 0 {
		maxSec = minSec
	}
	if maxSec < minSec {
		maxSec = minSec
	}
	if maxSec > 86400 {
		maxSec = 86400
	}
	return minSec, maxSec
}

func readAutoSenderPreset() (autoSenderPreset, bool) {
	file, err := readAutoSenderPresetsFile()
	if err != nil {
		return autoSenderPreset{}, false
	}
	active := strings.TrimSpace(file.Active)
	if active == "" {
		active = "default"
	}
	if preset, _, ok := findAutoSenderPreset(file, active); ok {
		return preset, true
	}
	return autoSenderPreset{}, false
}

func writeAutoSenderPreset(minSec, maxSec int) error {
	minSec, maxSec = sanitizeAutosendRange(minSec, maxSec)
	file, err := readAutoSenderPresetsFile()
	if err != nil {
		return err
	}
	preset, i, ok := findAutoSenderPreset(file, "default")
	if !ok {
		preset.Name = "default"
		file.Presets = append(file.Presets, preset)
		i = len(file.Presets) - 1
	}
	preset.MinSec = minSec
	preset.MaxSec = maxSec
	preset.UpdatedAt = time.Now().UTC()
	file.Presets[i] = preset
	file.Active = "default"
	return writeAutoSenderPresetsFile(file)
}

func copyIntSlice(in []int) []int {
	if len(in) == 0 {
		return nil
	}
	out := make([]int, len(in))
	copy(out, in)
	return out
}

func shuffleStrings(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	rand.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

func popShuffledMessage(deck *[]string, messages []string) (string, bool) {
	if len(messages) == 0 {
		*deck = nil
		return "", false
	}
	if len(*deck) == 0 {
		*deck = shuffleStrings(messages)
	}
	n := len(*deck)
	msg := (*deck)[n-1]
	*deck = (*deck)[:n-1]
	return msg, true
}

func (srv *webServer) nextAutosendMessage() (string, bool) {
	fileMessages := readAutoSenderMessages()
	srv.autosendMu.Lock()
	defer srv.autosendMu.Unlock()
	messages := fileMessages
	if len(srv.autosendMessages) > 0 {
		messages = srv.autosendMessages
	}
	if len(messages) == 0 {
		return "", false
	}
	if srv.adminRelease {
		return popShuffledMessage(&srv.autosendShuffleDeck, messages)
	}
	idx := srv.autosendNextIndex
	srv.autosendNextIndex = idx + 1
	return messages[idx%len(messages)], true
}

func autosendDelay(minSec, maxSec int) time.Duration {
	if maxSec <= minSec {
		return time.Duration(minSec) * time.Second
	}
	return time.Duration(minSec+rand.Intn(maxSec-minSec+1)) * time.Second
}

func waitAutosendDelay(ctx context.Context, stopCh <-chan struct{}, minSec, maxSec int) bool {
	timer := time.NewTimer(autosendDelay(minSec, maxSec))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-stopCh:
		return false
	case <-timer.C:
		return true
	}
}

func (srv *webServer) runAutosendSequence(stopCh <-chan struct{}, accountIDs []int, minSec, maxSec int) {
	if len(accountIDs) == 0 {
		return
	}
	nextAccount := 0
	for {
		select {
		case <-srv.ctx.Done():
			return
		case <-stopCh:
			return
		default:
		}

		msg, ok := srv.nextAutosendMessage()
		if !ok {
			if !waitAutosendDelay(srv.ctx, stopCh, minSec, maxSec) {
				return
			}
			continue
		}
		select {
		case <-stopCh:
			return
		default:
		}
		accountID := accountIDs[nextAccount]
		nextAccount = (nextAccount + 1) % len(accountIDs)
		if ok, reason := srv.manager.SendTask(accountID, srv.sendTaskForMessage(msg, "")); !ok {
			log.Printf("[autosend] account_id=%d skipped: %s", accountID, reason)
		}
		if !waitAutosendDelay(srv.ctx, stopCh, minSec, maxSec) {
			return
		}
	}
}

func (srv *webServer) validAccountPositions(accountIDs []int) []int {
	seen := make(map[int]bool, len(accountIDs))
	valid := make([]int, 0, len(accountIDs))
	for _, id := range accountIDs {
		if id < 1 || seen[id] {
			continue
		}
		if _, _, _, _, ok := srv.store.GetAccountByIndex(id); !ok {
			continue
		}
		seen[id] = true
		valid = append(valid, id)
	}
	return valid
}

func (srv *webServer) stopAutosendLocked() {
	if srv.autosendStopCh != nil {
		close(srv.autosendStopCh)
	}
	srv.autosendStopCh = nil
	srv.autosendEnabled = false
	srv.autosendAccountIDs = nil
	srv.autosendMessages = nil
	srv.autosendPresetName = ""
	srv.autosendShuffleDeck = nil
	srv.autosendNextIndex = 0
}

func (srv *webServer) stopAutosend() {
	srv.autosendMu.Lock()
	srv.stopAutosendLocked()
	srv.autosendMu.Unlock()
}

func (srv *webServer) startAutosend(accountIDs []int, messages []string, presetName string, minSec, maxSec int) error {
	accountIDs = srv.validAccountPositions(accountIDs)
	if len(accountIDs) == 0 {
		return fmt.Errorf("select at least one account")
	}
	if messages != nil && len(messages) == 0 {
		return fmt.Errorf("add at least one message")
	}
	minSec, maxSec = sanitizeAutosendRange(minSec, maxSec)
	stopCh := make(chan struct{})
	srv.autosendMu.Lock()
	srv.stopAutosendLocked()
	srv.autosendEnabled = true
	srv.autosendMinSec = minSec
	srv.autosendMaxSec = maxSec
	srv.autosendAccountIDs = copyIntSlice(accountIDs)
	srv.autosendMessages = append([]string(nil), messages...)
	srv.autosendPresetName = presetName
	srv.autosendStopCh = stopCh
	srv.autosendMu.Unlock()
	go srv.runAutosendSequence(stopCh, accountIDs, minSec, maxSec)
	return nil
}

func (srv *webServer) accountPositionsFromStableIDs(stableIDs []int) []int {
	seen := make(map[int]bool, len(stableIDs))
	positions := make([]int, 0, len(stableIDs))
	for _, stableID := range stableIDs {
		if stableID < 1 || seen[stableID] {
			continue
		}
		position, ok := srv.store.PositionByStableID(stableID)
		if !ok {
			continue
		}
		seen[stableID] = true
		positions = append(positions, position)
	}
	return positions
}

func (srv *webServer) handleAPIAutosend(rw http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		srv.autosendMu.Lock()
		enabled := srv.autosendEnabled
		minSec := srv.autosendMinSec
		maxSec := srv.autosendMaxSec
		accountIDs := copyIntSlice(srv.autosendAccountIDs)
		activePreset := srv.autosendPresetName
		activeMessageCount := len(srv.autosendMessages)
		srv.autosendMu.Unlock()
		if minSec == 0 {
			minSec = 60
		}
		if maxSec == 0 {
			maxSec = minSec
		}
		preset, hasPreset := readAutoSenderPreset()
		rw.Header().Set("Content-Type", "application/json")
		messageCount := len(readAutoSenderMessages())
		if activeMessageCount > 0 {
			messageCount = activeMessageCount
		}
		_ = json.NewEncoder(rw).Encode(map[string]interface{}{
			"enabled":        enabled,
			"min_sec":        minSec,
			"max_sec":        maxSec,
			"account_ids":    accountIDs,
			"messages_count": messageCount,
			"active_preset":  activePreset,
			"file":           autoSenderFileName,
			"preset_file":    autoSenderPresetsFileName,
			"preset": map[string]interface{}{
				"exists":  hasPreset,
				"min_sec": preset.MinSec,
				"max_sec": preset.MaxSec,
			},
		})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Enabled     bool  `json:"enabled"`
		IntervalSec int   `json:"interval_sec"`
		MinSec      int   `json:"min_sec"`
		MaxSec      int   `json:"max_sec"`
		AccountIDs  []int `json:"account_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(rw, "invalid json", http.StatusBadRequest)
		return
	}
	if body.IntervalSec > 0 && body.MinSec == 0 && body.MaxSec == 0 {
		body.MinSec = body.IntervalSec
		body.MaxSec = body.IntervalSec
	}
	minSec, maxSec := sanitizeAutosendRange(body.MinSec, body.MaxSec)
	if !body.Enabled {
		srv.stopAutosend()
	} else {
		if len(readAutoSenderMessages()) == 0 {
			http.Error(rw, autoSenderFileName+" is empty or missing", http.StatusBadRequest)
			return
		}
		if err := srv.startAutosend(body.AccountIDs, nil, "", minSec, maxSec); err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
	}

	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(map[string]bool{"ok": true})
}

func (srv *webServer) handleAPIAutosendPreset(rw http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		preset, ok := readAutoSenderPreset()
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]interface{}{
			"exists":      ok,
			"min_sec":     preset.MinSec,
			"max_sec":     preset.MaxSec,
			"file":        autoSenderPresetsFileName,
			"updated_at":  preset.UpdatedAt,
			"preset_name": preset.Name,
		})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		MinSec int `json:"min_sec"`
		MaxSec int `json:"max_sec"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(rw, "invalid json", http.StatusBadRequest)
		return
	}
	minSec, maxSec := sanitizeAutosendRange(body.MinSec, body.MaxSec)
	if err := writeAutoSenderPreset(minSec, maxSec); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(map[string]interface{}{
		"ok":      true,
		"min_sec": minSec,
		"max_sec": maxSec,
		"file":    autoSenderPresetsFileName,
	})
}

func (srv *webServer) handleAPIEventPresets(rw http.ResponseWriter, r *http.Request) {
	if !srv.adminRelease {
		http.Error(rw, "admin release only", http.StatusForbidden)
		return
	}
	file, err := readAutoSenderPresetsFile()
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case http.MethodGet:
		srv.autosendMu.Lock()
		active := srv.autosendPresetName
		srv.autosendMu.Unlock()
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]interface{}{
			"active":  active,
			"presets": file.Presets,
		})
	case http.MethodPost:
		var body struct {
			Action           string   `json:"action"`
			Name             string   `json:"name"`
			OriginalName     string   `json:"original_name"`
			Messages         []string `json:"messages"`
			AccountStableIDs []int    `json:"account_stable_ids"`
			MinSec           int      `json:"min_sec"`
			MaxSec           int      `json:"max_sec"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(rw, "invalid json", http.StatusBadRequest)
			return
		}
		name, err := normalizePresetName(body.Name)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		if strings.EqualFold(strings.TrimSpace(body.Action), "start") {
			preset, _, ok := findAutoSenderPreset(file, name)
			if !ok {
				http.Error(rw, "preset not found", http.StatusNotFound)
				return
			}
			messages, err := normalizePresetMessages(preset.Messages)
			if err != nil {
				http.Error(rw, err.Error(), http.StatusBadRequest)
				return
			}
			accountIDs := srv.accountPositionsFromStableIDs(preset.AccountStableIDs)
			if len(accountIDs) == 0 {
				http.Error(rw, "preset has no available accounts", http.StatusBadRequest)
				return
			}
			file.Active = preset.Name
			if err := writeAutoSenderPresetsFile(file); err != nil {
				http.Error(rw, err.Error(), http.StatusInternalServerError)
				return
			}
			if err := srv.startAutosend(accountIDs, messages, preset.Name, preset.MinSec, preset.MaxSec); err != nil {
				http.Error(rw, err.Error(), http.StatusBadRequest)
				return
			}
			rw.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(rw).Encode(map[string]interface{}{"ok": true, "active": preset.Name})
			return
		}

		messages, err := normalizePresetMessages(body.Messages)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		minSec, maxSec := sanitizeAutosendRange(body.MinSec, body.MaxSec)
		stableIDs := make([]int, 0, len(body.AccountStableIDs))
		seen := make(map[int]bool, len(body.AccountStableIDs))
		for _, stableID := range body.AccountStableIDs {
			if stableID < 1 || seen[stableID] {
				continue
			}
			if _, ok := srv.store.PositionByStableID(stableID); !ok {
				http.Error(rw, "preset contains an unknown account", http.StatusBadRequest)
				return
			}
			seen[stableID] = true
			stableIDs = append(stableIDs, stableID)
		}
		if len(stableIDs) == 0 {
			http.Error(rw, "select at least one account", http.StatusBadRequest)
			return
		}
		preset := autoSenderPreset{
			Name:             name,
			Messages:         messages,
			AccountStableIDs: stableIDs,
			MinSec:           minSec,
			MaxSec:           maxSec,
			UpdatedAt:        time.Now().UTC(),
		}
		targetIndex := -1
		if strings.TrimSpace(body.OriginalName) != "" {
			if _, i, ok := findAutoSenderPreset(file, body.OriginalName); ok {
				targetIndex = i
			}
		}
		if _, i, ok := findAutoSenderPreset(file, name); ok && i != targetIndex {
			http.Error(rw, "a preset with this name already exists", http.StatusConflict)
			return
		}
		if targetIndex >= 0 {
			file.Presets[targetIndex] = preset
		} else if _, i, ok := findAutoSenderPreset(file, name); ok {
			file.Presets[i] = preset
		} else {
			file.Presets = append(file.Presets, preset)
		}
		if err := writeAutoSenderPresetsFile(file); err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]interface{}{"ok": true, "preset": preset})
	case http.MethodDelete:
		name, err := normalizePresetName(r.URL.Query().Get("name"))
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		_, i, ok := findAutoSenderPreset(file, name)
		if !ok {
			http.Error(rw, "preset not found", http.StatusNotFound)
			return
		}
		deletedName := file.Presets[i].Name
		file.Presets = append(file.Presets[:i], file.Presets[i+1:]...)
		if strings.EqualFold(file.Active, deletedName) {
			file.Active = ""
		}
		if err := writeAutoSenderPresetsFile(file); err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		srv.autosendMu.Lock()
		if strings.EqualFold(srv.autosendPresetName, deletedName) {
			srv.stopAutosendLocked()
		}
		srv.autosendMu.Unlock()
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]bool{"ok": true})
	default:
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *webServer) handleAPIRunners(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	states := srv.manager.RunnerStates()
	runner.PublishAccountStates(states)
	type row struct {
		ID              int       `json:"id"`
		State           string    `json:"state"`
		QueueSize       int       `json:"queue_size"`
		LastSendTime    time.Time `json:"last_send_time"`
		CooldownUntil   time.Time `json:"cooldown_until"`
		CooldownSeconds int       `json:"cooldown_seconds"`
		SentTotal       int64     `json:"sent_total"`
		FailedTotal     int64     `json:"failed_total"`
	}
	var list []row
	for id, st := range states {
		cd := 0
		if st.CooldownUntil.After(time.Now()) {
			cd = int(time.Until(st.CooldownUntil).Seconds())
			if cd < 0 {
				cd = 0
			}
		}
		list = append(list, row{
			ID:              id,
			State:           st.State,
			QueueSize:       st.QueueSize,
			LastSendTime:    st.LastSendTime,
			CooldownUntil:   st.CooldownUntil,
			CooldownSeconds: cd,
			SentTotal:       st.SentTotal,
			FailedTotal:     st.FailedTotal,
		})
	}
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(list)
}

func streamUptimeSeconds(startTime string, now time.Time) int64 {
	started, err := time.Parse(time.RFC3339, strings.TrimSpace(startTime))
	if err != nil {
		started, err = time.Parse(time.RFC3339Nano, strings.TrimSpace(startTime))
	}
	if err != nil || started.After(now) {
		return 0
	}
	return int64(now.Sub(started).Seconds())
}

func streamStatusFromChannel(slug string, channel kickapitypes.ChannelData, now time.Time) streamStatus {
	return streamStatus{
		Slug:          slug,
		IsLive:        channel.Stream.IsLive,
		StartTime:     channel.Stream.StartTime,
		ViewerCount:   channel.Stream.ViewerCount,
		Title:         channel.StreamTitle,
		UptimeSeconds: streamUptimeSeconds(channel.Stream.StartTime, now),
		CheckedAt:     now.UTC(),
	}
}

func (srv *webServer) fetchStreamStatus() (streamStatus, error) {
	slug := srv.currentChannelSlug()
	if slug == "" {
		return streamStatus{}, fmt.Errorf("channel is not configured")
	}
	srv.streamStatusMu.Lock()
	if srv.streamStatusCached.Slug == slug && time.Since(srv.streamStatusCachedAt) < 15*time.Second {
		status := srv.streamStatusCached
		if status.IsLive {
			status.UptimeSeconds = streamUptimeSeconds(status.StartTime, time.Now())
		}
		srv.streamStatusMu.Unlock()
		return status, nil
	}
	srv.streamStatusMu.Unlock()

	ctx, cancel := context.WithTimeout(srv.ctx, 8*time.Second)
	defer cancel()
	var lastErr error
	for _, account := range srv.store.List() {
		token, _, proxy, _, ok := srv.store.GetAccountByIndex(account.Num)
		if !ok || strings.TrimSpace(token) == "" {
			continue
		}
		api, err := kick.NewAPIClient(kickapitypes.APIClientConfig{HTTPClient: apiClientWithProxy(proxy)})
		if err != nil {
			lastErr = err
			continue
		}
		channels, err := api.Channel().GetChannelsByBroadcasterSlug(ctx, token, []string{slug})
		if err != nil && proxy != "" && isNetworkOrProxyError(err) {
			api, _ = kick.NewAPIClient(kickapitypes.APIClientConfig{HTTPClient: http.DefaultClient})
			channels, err = api.Channel().GetChannelsByBroadcasterSlug(ctx, token, []string{slug})
		}
		if err != nil {
			lastErr = err
			continue
		}
		if len(channels.Data) == 0 {
			lastErr = fmt.Errorf("channel not found")
			continue
		}
		channel := channels.Data[0]
		now := time.Now()
		status := streamStatusFromChannel(slug, channel, now)
		srv.streamStatusMu.Lock()
		srv.streamStatusCached = status
		srv.streamStatusCachedAt = now
		srv.streamStatusMu.Unlock()
		return status, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no account token available")
	}
	return streamStatus{}, lastErr
}

func (srv *webServer) handleAPIStreamStatus(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	status, err := srv.fetchStreamStatus()
	if err != nil {
		http.Error(rw, "stream status unavailable", http.StatusBadGateway)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(status)
}

func (srv *webServer) handleAPIChannel(rw http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]interface{}{
			"slug":          srv.currentChannelSlug(),
			"admin_release": srv.adminRelease,
		})
	case http.MethodPut, http.MethodPost:
		var body struct {
			Slug        string `json:"slug"`
			ChannelSlug string `json:"channel_slug"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(rw, "invalid json", http.StatusBadRequest)
			return
		}
		rawSlug := body.ChannelSlug
		if rawSlug == "" {
			rawSlug = body.Slug
		}
		slug, err := normalizeChannelSlugInput(rawSlug)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		if err := writeChannelSlugToDotenv(slug); err != nil {
			log.Printf("channel: write .env: %v", err)
			http.Error(rw, "could not save .env", http.StatusInternalServerError)
			return
		}
		srv.switchChannel(slug)
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]string{"status": "ok", "slug": slug})
	default:
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// revalidateLicense calls the license server /validate; if status is not "active", sets gate to invalid.
// So when the user refreshes the dashboard after admin revoke, they get the license page again.
func (srv *webServer) revalidateLicense() {
	if srv.skipLicense || srv.licenseServerURL == "" || srv.licenseGate == nil || srv.licenseStore == nil {
		return
	}
	payload, err := srv.licenseStore.Load()
	if err != nil || payload == nil || payload.LicenseKey == "" {
		return
	}
	reqBody := map[string]string{
		"license_key": payload.LicenseKey,
		"hwid":        srv.deviceFingerprint,
	}
	reqJSON, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(srv.ctx, http.MethodPost, srv.licenseServerURL+"/validate", strings.NewReader(string(reqJSON)))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := licenseCheckClient.Do(req)
	if err != nil {
		return // таймаут или сеть — не меняем gate, чтобы не блокировать пользователя
	}
	defer resp.Body.Close()
	var result struct {
		Status string `json:"status"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result.Status != "active" {
		srv.licenseGate.SetValid(false)
	}
}

func (srv *webServer) kickConfigured() bool {
	return strings.TrimSpace(srv.clientID) != "" &&
		strings.TrimSpace(srv.clientSecret) != "" &&
		strings.TrimSpace(srv.currentChannelSlug()) != ""
}

func (srv *webServer) licenseAllowed() bool {
	if srv.skipLicense {
		return true
	}
	return srv.licenseGate != nil && srv.licenseGate.Valid() && srv.licenseServerURL != ""
}

func (srv *webServer) handleDashboard(rw http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/dashboard" && r.URL.Path != "/dashboard/" {
		http.NotFound(rw, r)
		return
	}
	if !srv.kickConfigured() {
		data, err := staticFS.ReadFile("static/setup.html")
		if err != nil {
			http.Error(rw, "setup page not found", http.StatusNotFound)
			return
		}
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = rw.Write(data)
		return
	}
	// При каждой загрузке дашборда перепроверяем лицензию (если отозвана — покажем страницу лицензии)
	srv.revalidateLicense()
	if !srv.licenseAllowed() {
		data, err := staticFS.ReadFile("static/license.html")
		if err != nil {
			http.Error(rw, "license page not found", http.StatusNotFound)
			return
		}
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = rw.Write(data)
		return
	}
	data, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(rw, "dashboard not found", http.StatusNotFound)
		return
	}
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = rw.Write(data)
}

const oauthWebStatePrefix = "web-"

var oauthPendingMu sync.Mutex
var oauthPending = make(map[string]string) // state -> pkceVerifier

func (srv *webServer) handleOAuthStart(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if srv.oauthClient == nil {
		srv.writeOAuthKickSetupNeededPage(rw)
		return
	}
	state := oauthWebStatePrefix + fmt.Sprintf("%d", time.Now().UnixNano())
	data, err := srv.oauthClient.InitiateAuthorization(srv.oauthRedirectURI, state, kickscopes.Scopes{kickscopes.ChannelRead, kickscopes.ChatWrite})
	if err != nil {
		log.Printf("OAuth start: %v", err)
		srv.writeOAuthCallbackErrorPage(rw, "Could not start OAuth", err.Error())
		return
	}
	oauthPendingMu.Lock()
	oauthPending[state] = data.PKCEVerifier
	oauthPendingMu.Unlock()
	// Без автоматического 302 на Kick — только страница с кнопкой (иначе фоллбек/вкладка не успевают).
	srv.writeOAuthAuthorizePage(rw, data.AuthorizationURL)
}

func (srv *webServer) handleOAuthCallback(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if oerr := strings.TrimSpace(r.URL.Query().Get("error")); oerr != "" {
		log.Printf("OAuth callback: error=%s description=%s", oerr, r.URL.Query().Get("error_description"))
		if strings.Contains(strings.ToLower(oerr), "redirect") {
			srv.writeOAuthInvalidRedirectPage(rw)
			return
		}
		desc := strings.TrimSpace(r.URL.Query().Get("error_description"))
		if desc == "" {
			desc = oerr
		}
		srv.writeOAuthCallbackErrorPage(rw, "OAuth", desc)
		return
	}
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		srv.writeOAuthCallbackErrorPage(rw, "OAuth", "No authorization code. Click Add account again.")
		return
	}
	oauthPendingMu.Lock()
	verifier, ok := oauthPending[state]
	delete(oauthPending, state)
	oauthPendingMu.Unlock()
	if !ok {
		srv.writeOAuthCallbackErrorPage(rw, "Session expired", "Open Add account again and sign in to Kick.")
		return
	}
	if srv.oauthClient == nil {
		srv.writeOAuthCallbackErrorPage(rw, "OAuth not configured", "Enter Client ID, Secret, and channel on the setup screen.")
		return
	}
	tokenResp, err := srv.oauthClient.ExchangeAuthorizationCode(srv.ctx, srv.oauthRedirectURI, code, verifier)
	if err != nil {
		log.Printf("OAuth exchange: %v", err)
		srv.writeOAuthCallbackErrorPage(rw, "Code exchange failed", "Ensure Redirect URL in Kick matches KICK_REDIRECT_URI (or the default dashboard URL). "+err.Error())
		return
	}
	if _, err := srv.store.Add("", tokenResp.AccessToken, tokenResp.RefreshToken); err != nil {
		log.Printf("Add account: %v", err)
		srv.writeOAuthCallbackErrorPage(rw, "Could not save account", err.Error())
		return
	}
	if err := srv.store.Save(); err != nil {
		log.Printf("Save accounts: %v", err)
	}
	srv.manager.EnsureRunner(srv.store.Count())
	srv.broadcasterMu.Lock()
	srv.broadcasterID = 0
	srv.broadcasterMu.Unlock()
	srv.writeOAuthSuccessPage(rw)
}

func (srv *webServer) oauthHTMLHead(title string) string {
	return fmt.Sprintf(`<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>%s</title>
<style>
body{font-family:system-ui,-apple-system,sans-serif;background:#0f0f0f;color:#efeff1;margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;padding:24px;box-sizing:border-box;}
.box{max-width:24rem;text-align:center;line-height:1.5;}
h1{font-size:1.1rem;color:#53fc18;margin:0 0 12px;font-weight:600;}
p{margin:0 0 14px;font-size:0.9rem;color:#adadb8;}
a.link{color:#53fc18;text-decoration:none;font-weight:500;}
a.link:hover{text-decoration:underline;}
a.btn{display:inline-block;margin-top:6px;padding:12px 20px;background:#53fc18;color:#0a0a0a!important;font-weight:600;text-decoration:none;border-radius:8px;}
</style></head><body><div class="box">`, template.HTMLEscapeString(title))
}

func (srv *webServer) oauthHTMLFoot() string {
	dash := template.HTMLEscapeString(strings.TrimSuffix(srv.dashboardBaseURL, "/") + "/")
	return fmt.Sprintf(`<p><a class="link" href="%s">Open dashboard</a></p></div></body></html>`, dash)
}

func (srv *webServer) writeOAuthAuthorizePage(rw http.ResponseWriter, authURL string) {
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	rw.WriteHeader(http.StatusOK)
	esc := template.HTMLEscapeString(authURL)
	fmt.Fprint(rw, srv.oauthHTMLHead("Kick sign-in"))
	fmt.Fprintf(rw, `<h1>Connect Kick</h1>
<p>Automatic redirect is disabled. When you are ready, click the button to open Kick sign-in.</p>
<p><a class="btn" href="%s">Continue to Kick</a></p>`, esc)
	fmt.Fprint(rw, srv.oauthHTMLFoot())
}

func (srv *webServer) writeOAuthInvalidRedirectPage(rw http.ResponseWriter) {
	need := template.HTMLEscapeString(srv.oauthRedirectURI)
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	rw.WriteHeader(http.StatusOK)
	fmt.Fprint(rw, srv.oauthHTMLHead("Redirect URL"))
	fmt.Fprintf(rw, `<h1>Invalid redirect URL</h1>
<p>Kick returned <code>invalid_redirect_uri</code>. In your app at developers.kick.com, Redirect URL must <strong>exactly</strong> match what SaturX uses:</p>
<p style="word-break:break-all;font-size:0.85rem;color:#efeff1;margin:12px 0;">%s</p>
<p>Without <code>KICK_REDIRECT_URI</code> in .env, the default is <code>http://localhost:&lt;port&gt;/oauth/callback</code> (port from <code>DASHBOARD_PORT</code>, default 8080). Use the same URL in Kick.</p>`, need)
	fmt.Fprint(rw, srv.oauthHTMLFoot())
}

func (srv *webServer) writeOAuthKickSetupNeededPage(rw http.ResponseWriter) {
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	rw.WriteHeader(http.StatusOK)
	fmt.Fprint(rw, srv.oauthHTMLHead("Setup required"))
	fmt.Fprint(rw, `<h1>Kick not configured</h1><p>Enter Client ID, Secret, and channel on the app setup page first.</p>`)
	fmt.Fprint(rw, srv.oauthHTMLFoot())
}

func (srv *webServer) writeOAuthLicenseNeededPage(rw http.ResponseWriter) {
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	rw.WriteHeader(http.StatusOK)
	fmt.Fprint(rw, srv.oauthHTMLHead("License required"))
	fmt.Fprint(rw, `<h1>License</h1><p>Activate your key on the dashboard first, then click Add account again.</p>`)
	fmt.Fprint(rw, srv.oauthHTMLFoot())
}

func (srv *webServer) writeOAuthCallbackErrorPage(rw http.ResponseWriter, title, detail string) {
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	rw.WriteHeader(http.StatusOK)
	fmt.Fprint(rw, srv.oauthHTMLHead(title))
	fmt.Fprintf(rw, `<h1>%s</h1><p>%s</p>`, template.HTMLEscapeString(title), template.HTMLEscapeString(detail))
	fmt.Fprint(rw, srv.oauthHTMLFoot())
}

func (srv *webServer) writeOAuthSuccessPage(rw http.ResponseWriter) {
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	rw.WriteHeader(http.StatusOK)
	fmt.Fprint(rw, srv.oauthHTMLHead("Done"))
	fmt.Fprint(rw, `<h1>Done</h1>
<p>Your Kick account was saved. Close this tab and refresh the dashboard (F5).</p>`)
	fmt.Fprint(rw, srv.oauthHTMLFoot())
}

func (srv *webServer) handleAPICurrent(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		AccountID int `json:"account_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.AccountID < 1 {
		http.Error(rw, "invalid json or account_id", http.StatusBadRequest)
		return
	}
	if !srv.store.SetCurrent(body.AccountID) {
		http.Error(rw, "account not found", http.StatusNotFound)
		return
	}
	if err := srv.store.Save(); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	_, name, _, _ := srv.store.Current()
	log.Printf("Switched to account #%d: %s", body.AccountID, name)
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(map[string]string{"status": "ok"})
}

func (srv *webServer) handleAPIAccountsSub(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/accounts/")
	path = strings.Trim(path, "/")
	var idStr, kind string
	switch {
	case strings.HasSuffix(path, "/proxy"):
		idStr = strings.TrimSuffix(path, "/proxy")
		kind = "proxy"
	case strings.HasSuffix(path, "/name"):
		idStr = strings.TrimSuffix(path, "/name")
		kind = "name"
	default:
		http.NotFound(rw, r)
		return
	}
	id, err := strconv.Atoi(strings.Trim(idStr, "/"))
	if err != nil || id < 1 {
		http.Error(rw, "invalid account id", http.StatusBadRequest)
		return
	}
	switch kind {
	case "proxy":
		var body struct {
			Proxy string `json:"proxy"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(rw, "invalid json", http.StatusBadRequest)
			return
		}
		if err := srv.store.SetProxy(id, body.Proxy); err != nil {
			http.Error(rw, err.Error(), http.StatusNotFound)
			return
		}
		srv.broadcasterMu.Lock()
		srv.broadcasterID = 0
		srv.broadcasterMu.Unlock()
	case "name":
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(rw, "invalid json", http.StatusBadRequest)
			return
		}
		if err := srv.store.SetName(id, body.Name); err != nil {
			http.Error(rw, err.Error(), http.StatusNotFound)
			return
		}
	}
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(map[string]string{"status": "ok"})
}

func (srv *webServer) handleAPISetup(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		KickClientID     string `json:"kick_client_id"`
		KickClientSecret string `json:"kick_client_secret"`
		ChannelSlug      string `json:"channel_slug"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(rw, "invalid json", http.StatusBadRequest)
		return
	}
	cid := strings.TrimSpace(body.KickClientID)
	sec := strings.TrimSpace(body.KickClientSecret)
	slug := strings.TrimSpace(body.ChannelSlug)
	if cid == "" || sec == "" || slug == "" {
		http.Error(rw, "kick_client_id, kick_client_secret, channel_slug required", http.StatusBadRequest)
		return
	}
	if err := mergeKickEnvIntoDotenv(cid, sec, slug); err != nil {
		log.Printf("setup: write .env: %v", err)
		http.Error(rw, "could not save .env", http.StatusInternalServerError)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(map[string]interface{}{"ok": true, "restarting": true})
	go func() {
		time.Sleep(400 * time.Millisecond)
		relaunchSelf()
	}()
}

func (srv *webServer) handleAPISetupStatus(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(map[string]bool{"configured": srv.kickConfigured()})
}

func (srv *webServer) handleAPIShutdown(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if srv.onShutdown == nil {
		http.Error(rw, "shutdown not configured", http.StatusNotImplemented)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write([]byte(`{"status":"ok"}`))
	if flusher, ok := rw.(http.Flusher); ok {
		flusher.Flush()
	}
	go srv.onShutdown()
}

func (srv *webServer) handleViewerBotStart(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		ChannelSlug string   `json:"channel_slug"`
		Target      int      `json:"target"`
		Proxies     []string `json:"proxies"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(rw, "invalid json", http.StatusBadRequest)
		return
	}
	slug := strings.TrimSpace(body.ChannelSlug)
	if slug == "" {
		slug = srv.currentChannelSlug()
	}
	if slug == "" {
		http.Error(rw, "channel_slug required or set CHANNEL_SLUG", http.StatusBadRequest)
		return
	}
	if err := srv.viewerBot.Start(slug, body.Target, body.Proxies); err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(map[string]string{"status": "ok"})
}

func (srv *webServer) handleViewerBotStop(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	srv.viewerBot.Stop()
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(map[string]string{"status": "ok"})
}

func (srv *webServer) handleViewerBotStatus(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	st := srv.viewerBot.GetStatus()
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(st)
}

func (srv *webServer) handleViewerBotLog(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	lines := srv.viewerBot.GetLog()
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(map[string]interface{}{"lines": lines})
}

// requireLicense returns 403 when license is not valid (unless SKIP_LICENSE test mode).
func (srv *webServer) requireLicense(h http.HandlerFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Cache-Control", "no-store")
		if !srv.licenseAllowed() {
			http.Error(rw, "license required", http.StatusForbidden)
			return
		}
		h(rw, r)
	}
}

// requireLicenseOAuth serves HTML instead of redirect so the OAuth tab never gets an instant Location bounce.
func (srv *webServer) requireLicenseOAuth(h http.HandlerFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Cache-Control", "no-store")
		if !srv.licenseAllowed() {
			srv.writeOAuthLicenseNeededPage(rw)
			return
		}
		h(rw, r)
	}
}

func (srv *webServer) handleLicenseActivate(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if srv.skipLicense {
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]bool{"ok": true})
		return
	}
	if srv.licenseServerURL == "" || srv.licenseGate == nil || srv.licenseStore == nil {
		http.Error(rw, "License server is not configured. Contact your vendor.", http.StatusServiceUnavailable)
		return
	}
	if srv.licenseGate.Valid() {
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]bool{"ok": true})
		return
	}
	var body struct {
		LicenseKey string `json:"license_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.LicenseKey == "" {
		http.Error(rw, "invalid request: license_key required", http.StatusBadRequest)
		return
	}
	reqBody := map[string]string{
		"license_key":        body.LicenseKey,
		"device_fingerprint": srv.deviceFingerprint,
	}
	reqJSON, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(srv.ctx, http.MethodPost, srv.licenseServerURL+"/activate", strings.NewReader(string(reqJSON)))
	if err != nil {
		log.Printf("license activate request: %v", err)
		http.Error(rw, "internal error", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("license activate: %v", err)
		http.Error(rw, "license server unreachable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	var result struct {
		Status        string `json:"status"`
		Error         string `json:"error"`
		RefreshToken  string `json:"refresh_token"`
		SignedLicense string `json:"signed_license"`
		DeviceID      string `json:"device_id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if resp.StatusCode != http.StatusOK {
		msg := result.Error
		if msg == "" {
			msg = "activation failed"
		}
		http.Error(rw, msg, resp.StatusCode)
		return
	}
	if result.SignedLicense == "" || result.RefreshToken == "" || result.DeviceID == "" {
		http.Error(rw, "invalid server response", http.StatusInternalServerError)
		return
	}
	if err := srv.licenseStore.Save(&licensestore.Payload{
		LicenseKey:       body.LicenseKey,
		SignedLicense:    result.SignedLicense,
		RefreshToken:     result.RefreshToken,
		DeviceID:         result.DeviceID,
		LastValidationAt: time.Now().UTC(),
	}); err != nil {
		log.Printf("license save: %v", err)
		http.Error(rw, "failed to save license", http.StatusInternalServerError)
		return
	}
	srv.licenseGate.SetValid(true)
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(map[string]bool{"ok": true})
}

func (srv *webServer) sendTaskForMessage(message, replyToMessageID string) runner.SendTask {
	return runner.SendTask{
		Message:          message,
		ReplyToMessageID: strings.TrimSpace(replyToMessageID),
		AllowDuplicate:   srv.adminRelease || isKnownKickEmojiContent(message),
	}
}

func runWebServer(ctx context.Context, store *accountsStore, channelSlug string, port string, manager *runner.AccountManager, vb *viewerbot.Bot, oauthClient kickcontracts.OAuthClient, clientID, clientSecret string, onChannelChanged func(string), shutdownFn func(), gate *licenseGate, licStore *licensestore.Store, licenseURL, licenseSecret, deviceFP string, skipLicense, adminRelease bool) {
	if port == "" {
		port = "8080"
	}
	dashboardBaseURL := "http://localhost:" + port
	defaultOAuthCallback := dashboardBaseURL + "/oauth/callback"
	oauthRedirect := strings.TrimSpace(os.Getenv("KICK_REDIRECT_URI"))
	if oauthRedirect == "" {
		oauthRedirect = defaultOAuthCallback
	}
	u, err := url.Parse(oauthRedirect)
	if err != nil || u.Scheme == "" || u.Host == "" {
		log.Printf("OAuth: invalid KICK_REDIRECT_URI, using %s", defaultOAuthCallback)
		oauthRedirect = defaultOAuthCallback
		u, _ = url.Parse(oauthRedirect)
	}
	log.Printf("OAuth redirect_uri for Kick app: %s (set KICK_REDIRECT_URI to override)", oauthRedirect)
	callbackPath := u.Path
	if callbackPath == "" {
		callbackPath = "/"
	}
	dashU, _ := url.Parse(dashboardBaseURL)
	sameAddr := strings.EqualFold(u.Scheme, dashU.Scheme) && strings.EqualFold(u.Host, dashU.Host)

	srv := &webServer{
		ctx:               ctx,
		store:             store,
		manager:           manager,
		viewerBot:         vb,
		onChannelChanged:  onChannelChanged,
		onShutdown:        shutdownFn,
		oauthClient:       oauthClient,
		clientID:          clientID,
		clientSecret:      clientSecret,
		oauthRedirectURI:  oauthRedirect,
		dashboardBaseURL:  dashboardBaseURL,
		licenseGate:       gate,
		licenseStore:      licStore,
		licenseServerURL:  licenseURL,
		licenseHMACSecret: licenseSecret,
		deviceFingerprint: deviceFP,
		skipLicense:       skipLicense,
		adminRelease:      adminRelease,
	}
	srv.switchChannel(channelSlug)
	mux := http.NewServeMux()
	if emotesFS, err := fs.Sub(staticFS, "static/emotes"); err == nil {
		mux.Handle("/emotes/", http.StripPrefix("/emotes/", http.FileServer(http.FS(emotesFS))))
	}
	mux.HandleFunc("/", srv.handleDashboard)
	mux.HandleFunc("/license/activate", srv.handleLicenseActivate)
	mux.HandleFunc("/api/setup", srv.handleAPISetup)
	mux.HandleFunc("/api/setup/status", srv.handleAPISetupStatus)
	mux.HandleFunc("/oauth/start", srv.requireLicenseOAuth(srv.handleOAuthStart))
	mux.HandleFunc("/oauth/callback", srv.requireLicenseOAuth(srv.handleOAuthCallback))
	if sameAddr && callbackPath != "/oauth/callback" {
		mux.HandleFunc(callbackPath, srv.requireLicenseOAuth(srv.handleOAuthCallback))
		log.Printf("OAuth: registered KICK_REDIRECT_URI path %s on dashboard port", callbackPath)
	}
	if !sameAddr && u.Port() != "" {
		addr := ":" + u.Port()
		legacyMux := http.NewServeMux()
		legacyMux.HandleFunc(callbackPath, srv.requireLicenseOAuth(srv.handleOAuthCallback))
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			log.Printf("OAuth: cannot bind legacy callback %s: %v", addr, err)
		} else {
			log.Printf("OAuth: legacy callback ready on %s (%s)", addr, oauthRedirect)
			go func() {
				if err := http.Serve(ln, legacyMux); err != nil {
					log.Printf("OAuth callback listener: %v", err)
				}
			}()
		}
	}
	mux.HandleFunc("/api/accounts", srv.requireLicense(srv.handleAPIAccounts))
	mux.HandleFunc("/api/accounts/current", srv.requireLicense(srv.handleAPICurrent))
	mux.HandleFunc("/api/accounts/order", srv.requireLicense(srv.handleAPIAccountsOrder))
	mux.HandleFunc("/api/accounts/", srv.requireLicense(srv.handleAPIAccountsSub))
	mux.HandleFunc("/api/status", srv.requireLicense(srv.handleAPIStatus))
	mux.HandleFunc("/api/runners", srv.requireLicense(srv.handleAPIRunners))
	mux.HandleFunc("/api/send", srv.requireLicense(srv.handleAPISend))
	mux.HandleFunc("/api/chat/history", srv.requireLicense(srv.handleAPIChatHistory))
	mux.HandleFunc("/api/chat/events", srv.requireLicense(srv.handleAPIChatEvents))
	mux.HandleFunc("/api/emotes", srv.requireLicense(srv.handleAPIEmotes))
	mux.HandleFunc("/api/messages", srv.requireLicense(srv.handleAPIMessages))
	mux.HandleFunc("/api/autosend", srv.requireLicense(srv.handleAPIAutosend))
	mux.HandleFunc("/api/autosend/preset", srv.requireLicense(srv.handleAPIAutosendPreset))
	mux.HandleFunc("/api/autosend/presets", srv.requireLicense(srv.handleAPIEventPresets))
	mux.HandleFunc("/api/channel", srv.requireLicense(srv.handleAPIChannel))
	mux.HandleFunc("/api/stream/status", srv.requireLicense(srv.handleAPIStreamStatus))
	mux.HandleFunc("/api/shutdown", srv.requireLicense(srv.handleAPIShutdown))
	mux.HandleFunc("/api/viewerbot/start", srv.requireLicense(srv.handleViewerBotStart))
	mux.HandleFunc("/api/viewerbot/stop", srv.requireLicense(srv.handleViewerBotStop))
	mux.HandleFunc("/api/viewerbot/status", srv.requireLicense(srv.handleViewerBotStatus))
	mux.HandleFunc("/api/viewerbot/log", srv.requireLicense(srv.handleViewerBotLog))
	addr := ":" + port
	log.Printf("Dashboard: http://localhost%s", addr)
	_ = http.ListenAndServe(addr, mux)
}
