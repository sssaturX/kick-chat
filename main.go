// SaturX — отправка сообщений в чат через OAuth (kick-go-sdk).
// Регистрация приложения: https://developers.kick.com/
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"kick-chat-go/internal/fingerprint"
	"kick-chat-go/internal/httpclient"
	"kick-chat-go/internal/licensestore"
	"kick-chat-go/internal/runner"
	"kick-chat-go/internal/viewerbot"

	"github.com/henrikah/kick-go-sdk/v2"
	"github.com/henrikah/kick-go-sdk/v2/kickapitypes"
	"github.com/henrikah/kick-go-sdk/v2/kickcontracts"
	"github.com/henrikah/kick-go-sdk/v2/kickoauthtypes"
	"github.com/joho/godotenv"
)

// Значения по умолчанию для лицензии — задаются при сборке через -ldflags.
// Юзеру не нужно прописывать LICENSE_SERVER_URL и LICENSE_HMAC_SECRET в .env.
// defaultAdminRelease=1 — отдельная админ-сборка (scripts/build-release-admin.*), лицензия отключена.
var (
	defaultLicenseServerURL  string
	defaultLicenseHMACSecret string
	defaultAdminRelease      string
)

func apiClientWithProxy(proxyStr string) *http.Client {
	return httpFactory.Get(proxyStr)
}

// httpStatusInError находит первый HTTP-код 4xx/5xx в тексте ошибки (Kick/SDK часто вшивают "403", "400" и т.д.).
var httpStatusInError = regexp.MustCompile(`\b([45][0-9]{2})\b`)

func statusCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	m := httpStatusInError.FindStringSubmatch(err.Error())
	if len(m) < 2 {
		return 0
	}
	c, e := strconv.Atoi(m[1])
	if e != nil || c < 400 || c > 599 {
		return 0
	}
	return c
}

var httpFactory *httpclient.Factory
var channelSlugValue = regexp.MustCompile(`^[A-Za-z0-9_-]{2,64}$`)

const broadcasterResolveTimeout = 5 * time.Second

func normalizeChannelSlugInput(input string) (string, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return "", errors.New("channel_slug required")
	}
	if u, err := url.Parse(s); err == nil && u.Host != "" {
		s = strings.Trim(strings.TrimPrefix(u.Path, "/"), "/")
		if i := strings.Index(s, "/"); i >= 0 {
			s = s[:i]
		}
	}
	s = strings.TrimPrefix(strings.TrimSpace(s), "@")
	s = strings.Trim(s, "/")
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	if !channelSlugValue.MatchString(s) {
		return "", errors.New("channel_slug must be 2-64 chars: letters, numbers, _ or -")
	}
	return s, nil
}

// Known Kick emote buttons and their native chat tokens.
type kickEmojiDef struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name"`
	Text  string `json:"text,omitempty"`
	Image string `json:"image,omitempty"`
}

var kickEmojiShortcodes = []kickEmojiDef{
	{ID: "37226", Name: "KEKW"},
	{ID: "37231", Name: "PatrickBoo"},
	{ID: "37236", Name: "ThisIsFine"},
	{ID: "37244", Name: "modCheck"},
	{ID: "39273", Name: "MuteD"},
	{ID: "37239", Name: "WeSmart"},
	{ID: "37233", Name: "PogU"},
	{ID: "37227", Name: "LULW"},
	{ID: "37218", Name: "Clap"},
	{ID: "37224", Name: "GIGACHAD"},
}

var kickNativeEmoteToken = regexp.MustCompile(`^\[emote:(\d+):([^\]]+)\]$`)

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func normalizeKickEmojiDefs(defs []kickEmojiDef) []kickEmojiDef {
	out := make([]kickEmojiDef, 0, len(defs))
	for _, def := range defs {
		def.ID = digitsOnly(strings.TrimSpace(def.ID))
		def.Name = strings.TrimSpace(def.Name)
		def.Text = strings.TrimSpace(def.Text)
		def.Image = strings.TrimSpace(def.Image)
		if def.Text != "" && (def.ID == "" || def.Name == "") {
			if m := kickNativeEmoteToken.FindStringSubmatch(def.Text); len(m) == 3 {
				if def.ID == "" {
					def.ID = m[1]
				}
				if def.Name == "" {
					def.Name = m[2]
				}
			}
		}
		if def.Name == "" {
			continue
		}
		if def.Text == "" && def.ID != "" {
			def.Text = fmt.Sprintf("[emote:%s:%s]", def.ID, def.Name)
		}
		if def.Image == "" && def.ID != "" {
			def.Image = "https://files.kick.com/emotes/" + def.ID + "/fullsize"
		}
		out = append(out, def)
	}
	return out
}

func loadKickEmojiConfig(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		kickEmojiShortcodes = normalizeKickEmojiDefs(kickEmojiShortcodes)
		return
	}
	var wrapper struct {
		Emotes []kickEmojiDef `json:"emotes"`
	}
	if err := json.Unmarshal(data, &wrapper); err == nil && len(wrapper.Emotes) > 0 {
		if defs := normalizeKickEmojiDefs(wrapper.Emotes); len(defs) > 0 {
			kickEmojiShortcodes = defs
			return
		}
	}
	var defs []kickEmojiDef
	if err := json.Unmarshal(data, &defs); err == nil {
		if normalized := normalizeKickEmojiDefs(defs); len(normalized) > 0 {
			kickEmojiShortcodes = normalized
			return
		}
	}
	kickEmojiShortcodes = normalizeKickEmojiDefs(kickEmojiShortcodes)
}

func kickEmojiDefinitions() []kickEmojiDef {
	defs := normalizeKickEmojiDefs(kickEmojiShortcodes)
	out := make([]kickEmojiDef, len(defs))
	copy(out, defs)
	return out
}

// normalizeKickEmojiContent keeps Kick emotes in Kick's native token form.
func normalizeKickEmojiContent(content string) string {
	s := strings.TrimSpace(content)
	for _, emoji := range kickEmojiDefinitions() {
		if s == emoji.Name || s == ":"+emoji.Name+":" || s == emoji.Text {
			return emoji.Text
		}
	}
	return content
}

func isKnownKickEmojiContent(content string) bool {
	s := strings.TrimSpace(content)
	if len(s) >= 3 && s[0] == ':' && s[len(s)-1] == ':' {
		s = s[1 : len(s)-1]
	}
	for _, emoji := range kickEmojiDefinitions() {
		if s == emoji.Name || s == emoji.Text {
			return true
		}
	}
	return false
}

const kickTokenURL = "https://id.kick.com/oauth/token"

// mergeKickEnvIntoDotenv writes KICK_CLIENT_ID, KICK_CLIENT_SECRET, CHANNEL_SLUG into .env (replaces existing keys).
func mergeKickEnvIntoDotenv(clientID, clientSecret, channelSlug string) error {
	envPath := ".env"
	data, _ := os.ReadFile(envPath)
	lines := strings.Split(string(data), "\n")
	var out []string
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "KICK_CLIENT_ID=") ||
			strings.HasPrefix(t, "KICK_CLIENT_SECRET=") ||
			strings.HasPrefix(t, "CHANNEL_SLUG=") {
			continue
		}
		out = append(out, l)
	}
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	out = append(out, "")
	out = append(out, "# Kick OAuth (edit in dashboard or here)")
	out = append(out, "KICK_CLIENT_ID="+clientID)
	out = append(out, "KICK_CLIENT_SECRET="+clientSecret)
	out = append(out, "CHANNEL_SLUG="+channelSlug)
	return os.WriteFile(envPath, []byte(strings.Join(out, "\n")), 0644)
}

func writeChannelSlugToDotenv(channelSlug string) error {
	envPath := ".env"
	data, _ := os.ReadFile(envPath)
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines)+2)
	wrote := false
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "CHANNEL_SLUG=") {
			if !wrote {
				out = append(out, "CHANNEL_SLUG="+channelSlug)
				wrote = true
			}
			continue
		}
		out = append(out, line)
	}
	if !wrote {
		for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
			out = out[:len(out)-1]
		}
		out = append(out, "", "# Optional: channel slug for chat", "CHANNEL_SLUG="+channelSlug)
	}
	return os.WriteFile(envPath, []byte(strings.Join(out, "\n")), 0644)
}

func relaunchSelf() {
	exe, err := os.Executable()
	if err != nil {
		log.Printf("relaunch: %v", err)
		return
	}
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Printf("relaunch: %v", err)
		return
	}
	os.Exit(0)
}

// doLicenseRefresh вызывает /license/refresh на сервере. При успехе обновляет хранилище и возвращает true; при отзыве/ошибке — false.
func doLicenseRefresh(ctx context.Context, licStore *licensestore.Store, licenseServerURL, _ string) bool {
	payload, err := licStore.Load()
	if err != nil || payload == nil || payload.RefreshToken == "" || payload.DeviceID == "" {
		return false
	}
	refreshBody, _ := json.Marshal(map[string]string{
		"refresh_token": payload.RefreshToken,
		"device_id":     payload.DeviceID,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, licenseServerURL+"/license/refresh", strings.NewReader(string(refreshBody)))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var ref struct {
		SignedLicense string `json:"signed_license"`
		RefreshToken  string `json:"refresh_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ref); err != nil || ref.SignedLicense == "" {
		return false
	}
	newRefresh := ref.RefreshToken
	if newRefresh == "" {
		newRefresh = payload.RefreshToken
	}
	if err := licStore.Save(&licensestore.Payload{
		LicenseKey:       payload.LicenseKey,
		SignedLicense:    ref.SignedLicense,
		RefreshToken:     newRefresh,
		DeviceID:         payload.DeviceID,
		LastValidationAt: time.Now().UTC(),
	}); err != nil {
		return false
	}
	return true
}

// doLicenseValidate вызывает POST /validate на сервере лицензий. Возвращает true только если status == "active".
func doLicenseValidate(ctx context.Context, licStore *licensestore.Store, licenseServerURL, deviceFP string) bool {
	payload, err := licStore.Load()
	if err != nil || payload == nil || payload.LicenseKey == "" {
		return false
	}
	reqBody, _ := json.Marshal(map[string]string{
		"license_key": payload.LicenseKey,
		"hwid":        deviceFP,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, licenseServerURL+"/validate", strings.NewReader(string(reqBody)))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var result struct {
		Status string `json:"status"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	return result.Status == "active"
}

// refreshAccessToken обменивает refresh_token на новую пару access+refresh. Тот же endpoint, что и code exchange.
func refreshAccessToken(ctx context.Context, clientID, clientSecret, refreshToken string, httpClient *http.Client) (accessToken, newRefreshToken string, err error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("refresh_token", refreshToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, kickTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("refresh token: %s: %s", resp.Status, string(body))
	}
	var data struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", "", err
	}
	return data.AccessToken, data.RefreshToken, nil
}

func resolveBroadcasterUserID(ctx context.Context, factory *httpclient.Factory, channelSlug, token, proxyStr string) int {
	if strings.TrimSpace(channelSlug) == "" || strings.TrimSpace(token) == "" {
		return 0
	}
	try := func(proxyKey string) (int, error) {
		client := http.DefaultClient
		if factory != nil {
			if c := factory.Get(proxyKey); c != nil {
				client = c
			}
		}
		api, err := kick.NewAPIClient(kickapitypes.APIClientConfig{HTTPClient: client})
		if err != nil {
			return 0, err
		}
		resolveCtx, cancel := context.WithTimeout(ctx, broadcasterResolveTimeout)
		defer cancel()
		channels, err := api.Channel().GetChannelsByBroadcasterSlug(resolveCtx, token, []string{channelSlug})
		if err != nil {
			return 0, err
		}
		if len(channels.Data) == 0 {
			return 0, nil
		}
		return channels.Data[0].BroadcasterUserID, nil
	}
	id, err := try(proxyStr)
	if err != nil && proxyStr != "" && isNetworkOrProxyError(err) {
		id, _ = try("")
	}
	return id
}

// makeSendFunc возвращает runner.SendFunc. При 401 вызывает refreshFunc (если есть) и один раз повторяет отправку.
func makeSendFunc(store *accountsStore, factory *httpclient.Factory, ctx context.Context, getChannelSlug func() string, getBID func() int, setBID func(int), refreshFunc func(accountID int) (newAccessToken string, err error)) runner.SendFunc {
	return func(accountID int, task runner.SendTask) runner.SendResult {
		channelSlug := strings.TrimSpace(getChannelSlug())
		if channelSlug == "" {
			return runner.SendResult{Err: errors.New("channel not configured"), StatusCode: 400}
		}
		tok, _, proxy, _, ok := store.GetAccountByIndex(accountID)
		if !ok {
			return runner.SendResult{Err: errors.New("account not found")}
		}
		client := factory.Get(proxy)
		if client == nil {
			client = http.DefaultClient
		}
		api, err := kick.NewAPIClient(kickapitypes.APIClientConfig{HTTPClient: client})
		if err != nil {
			return runner.SendResult{Err: err}
		}
		bid := getBID()
		if bid == 0 {
			bid = resolveBroadcasterUserID(ctx, factory, channelSlug, tok, proxy)
			if bid != 0 && setBID != nil {
				setBID(bid)
			}
		}
		if bid == 0 {
			return runner.SendResult{Err: errors.New("channel not resolved"), StatusCode: 503}
		}
		// The Send Chat API accepts text content only, so do not send raw Kick shortcode names.
		content := normalizeKickEmojiContent(task.Message)
		replyTo := strings.TrimSpace(task.ReplyToMessageID)
		var replyToPtr *string
		if replyTo != "" {
			replyToPtr = &replyTo
		}
		// Короткий таймаут для отправки: при зависшем прокси не ждём 30 сек, быстро переходим на прямое соединение.
		sendCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		start := time.Now()
		_, err = api.Chat().SendChatMessageAsUser(sendCtx, tok, bid, replyToPtr, content)
		if err != nil && proxy != "" && isNetworkOrProxyError(err) {
			cancel()
			sendCtx, cancel = context.WithTimeout(ctx, 8*time.Second)
			api, _ = kick.NewAPIClient(kickapitypes.APIClientConfig{HTTPClient: factory.Get("")})
			if api != nil {
				_, err = api.Chat().SendChatMessageAsUser(sendCtx, tok, bid, replyToPtr, content)
			}
		}
		// При 401 пробуем обновить токен и повторить один раз
		if err != nil && statusCodeFromError(err) == 401 && refreshFunc != nil {
			newTok, refreshErr := refreshFunc(accountID)
			if refreshErr == nil && newTok != "" {
				tok = newTok
				cancel()
				sendCtx, cancel = context.WithTimeout(ctx, 8*time.Second)
				_, err = api.Chat().SendChatMessageAsUser(sendCtx, tok, bid, replyToPtr, content)
				if err != nil && proxy != "" && isNetworkOrProxyError(err) {
					api, _ = kick.NewAPIClient(kickapitypes.APIClientConfig{HTTPClient: factory.Get("")})
					if api != nil {
						_, err = api.Chat().SendChatMessageAsUser(sendCtx, tok, bid, replyToPtr, content)
					}
				}
			}
		}
		cancel()
		latency := time.Since(start).Nanoseconds()
		if err != nil {
			runner.RecordSendFailure()
			code := statusCodeFromError(err)
			log.Printf("[send] account_id=%d channel=%s broadcaster_id=%d http=%d err=%v", accountID, channelSlug, bid, code, err)
			return runner.SendResult{Err: err, StatusCode: code}
		}
		runner.RecordSendSuccess(latency)
		return runner.SendResult{}
	}
}

func main() {
	_ = godotenv.Load()
	loadKickEmojiConfig("kick-emotes.json")

	// Логи только в файл (не в консоль). Файл: LOG_FILE или saturx.log
	logPath := os.Getenv("LOG_FILE")
	if logPath == "" {
		logPath = "saturx.log"
	}
	if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		log.SetOutput(f)
		log.SetFlags(log.Ldate | log.Ltime)
	}
	// при ошибке открытия файла логи остаются в stderr
	clientID := strings.TrimSpace(os.Getenv("KICK_CLIENT_ID"))
	clientSecret := strings.TrimSpace(os.Getenv("KICK_CLIENT_SECRET"))
	channelSlug := strings.TrimSpace(os.Getenv("CHANNEL_SLUG"))
	if channelSlug != "" {
		if normalized, err := normalizeChannelSlugInput(channelSlug); err == nil {
			channelSlug = normalized
		}
	}

	kickReady := clientID != "" && clientSecret != "" && channelSlug != ""
	if !kickReady {
		log.Println("Kick OAuth not configured — open the dashboard in your browser to enter Client ID, Secret, and channel slug.")
	}

	ctx := context.Background()
	var oauthClient kickcontracts.OAuthClient
	if kickReady {
		var err error
		oauthClient, err = kick.NewOAuthClient(kickoauthtypes.OAuthClientConfig{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			HTTPClient:   http.DefaultClient,
		})
		if err != nil {
			log.Fatalf("OAuth client: %v", err)
		}
	}

	store := newAccountsStore("")
	if err := store.Load(); err != nil {
		log.Fatalf("Load accounts: %v", err)
	}
	// Assign proxies from .kick_proxies to accounts 1..N (one line per account)
	if data, err := os.ReadFile(".kick_proxies"); err == nil {
		lines := strings.Split(string(data), "\n")
		store.assignProxiesFromLines(lines)
		_ = store.Save()
	}
	// Backward compat: if env or old file has token and store is empty, add one account
	if store.Count() == 0 {
		accessToken := os.Getenv("KICK_ACCESS_TOKEN")
		if accessToken == "" {
			if tokenBytes, err := os.ReadFile(".kick_access_token"); err == nil {
				accessToken = strings.TrimSpace(string(tokenBytes))
			}
		}
		if accessToken != "" {
			store.Add("", accessToken, "")
			_ = store.Save()
			log.Println("Imported token as account 1. Accounts saved to", accountsFile)
		}
	}
	if store.Count() == 0 && kickReady {
		log.Println("No Kick accounts yet — add one in the dashboard (Add account).")
	}
	// Ensure LastUsed is set if we have accounts
	if _, _, _, ok := store.Current(); !ok && store.Count() > 0 {
		store.Use(1)
		_ = store.Save()
	}

	httpFactory = httpclient.NewFactory()
	var channelMu sync.RWMutex
	var broadcasterUserID int
	getChannelSlug := func() string {
		channelMu.RLock()
		defer channelMu.RUnlock()
		return channelSlug
	}
	getBID := func() int {
		channelMu.RLock()
		defer channelMu.RUnlock()
		return broadcasterUserID
	}
	setBID := func(id int) {
		channelMu.Lock()
		broadcasterUserID = id
		channelMu.Unlock()
	}
	setChannelSlug := func(slug string) {
		channelMu.Lock()
		channelSlug = slug
		broadcasterUserID = 0
		channelMu.Unlock()
	}
	refreshFunc := func(accountID int) (string, error) {
		_, refresh, _, _, ok := store.GetAccountByIndex(accountID)
		if !ok || refresh == "" {
			return "", errors.New("no refresh token")
		}
		newAccess, newRefresh, err := refreshAccessToken(ctx, clientID, clientSecret, refresh, httpFactory.Get(""))
		if err != nil {
			log.Printf("[refresh] account %d: %v", accountID, err)
			return "", err
		}
		if err := store.UpdateToken(accountID, newAccess, newRefresh); err != nil {
			return "", err
		}
		log.Printf("[refresh] account %d: token renewed", accountID)
		return newAccess, nil
	}
	// При старте обновляем токены у всех аккаунтов, у которых есть refresh_token
	if oauthClient != nil {
		for i := 1; i <= store.Count(); i++ {
			_, refresh, _, _, ok := store.GetAccountByIndex(i)
			if ok && refresh != "" {
				if _, err := refreshFunc(i); err == nil {
					// уже залогировано в refreshFunc
				}
				if i < store.Count() {
					time.Sleep(300 * time.Millisecond)
				}
			}
		}
	}
	sendFunc := makeSendFunc(store, httpFactory, ctx, getChannelSlug, getBID, setBID, refreshFunc)
	manager := runner.NewAccountManager(sendFunc)
	for i := 1; i <= store.Count(); i++ {
		manager.EnsureRunner(i)
	}
	go runner.PublishAccountStates(manager.RunnerStates())

	dashboardPort := os.Getenv("DASHBOARD_PORT")
	if dashboardPort == "" {
		dashboardPort = "8080"
	}
	licenseServerURL := os.Getenv("LICENSE_SERVER_URL")
	if licenseServerURL == "" {
		licenseServerURL = defaultLicenseServerURL
	}
	licenseHMACSecret := os.Getenv("LICENSE_HMAC_SECRET")
	if licenseHMACSecret == "" {
		licenseHMACSecret = defaultLicenseHMACSecret
	}
	adminRelease := strings.TrimSpace(defaultAdminRelease) == "1"
	if adminRelease {
		licenseServerURL = ""
		licenseHMACSecret = ""
	}
	var gate *licenseGate
	var licStore *licensestore.Store
	deviceFP := ""
	if licenseServerURL != "" {
		deviceFP = fingerprint.Device()
		licStore = licensestore.NewStore(".kick_license.dat", deviceFP)
		gate = &licenseGate{}
		payload, loadErr := licStore.Load()
		if loadErr != nil {
			log.Printf("License load: %v", loadErr)
		}
		if payload != nil && licenseHMACSecret != "" {
			_, verifyErr := licensestore.VerifySignedLicense(licenseHMACSecret, payload.SignedLicense, payload.DeviceID)
			if verifyErr != nil {
				log.Printf("License verify: %v", verifyErr)
				_ = licStore.Delete()
			} else {
				// Всегда проверяем лицензию на сервере при старте (отзыв админом = отказ доступа)
				if doLicenseRefresh(ctx, licStore, licenseServerURL, licenseHMACSecret) {
					gate.SetValid(true)
				} else {
					log.Println("License refresh failed or revoked; re-activate in dashboard")
				}
			}
		}
	} else {
		// Без LICENSE_SERVER_URL доступ закрыт (или SKIP_LICENSE=1 для теста)
		gate = &licenseGate{}
	}
	skipLicense := adminRelease ||
		os.Getenv("SKIP_LICENSE") == "1" ||
		strings.ToLower(strings.TrimSpace(os.Getenv("SKIP_LICENSE"))) == "true"
	if skipLicense {
		gate.SetValid(true)
		if adminRelease {
			log.Println("Admin release build: license disabled")
		} else {
			log.Println("Test mode: SKIP_LICENSE=1 — license checks disabled")
		}
	}
	// При старте без валидной лицензии не работаем: останавливаем раннеры (они уже созданы выше)
	if licenseServerURL != "" && gate != nil && !gate.Valid() && !skipLicense {
		manager.Stop()
	}
	vb := viewerbot.New()
	go runWebServer(ctx, store, channelSlug, dashboardPort, manager, vb, oauthClient, clientID, clientSecret, setChannelSlug, func() {
		time.Sleep(200 * time.Millisecond)
		os.Exit(0)
	}, gate, licStore, licenseServerURL, licenseHMACSecret, deviceFP, skipLicense, adminRelease)

	// При каждом запуске открываем дашборд в браузере после поднятия веб-сервера.
	go func() {
		time.Sleep(650 * time.Millisecond)
		minimizeConsoleIfPossible()
		openDashboardBrowser("http://localhost:" + dashboardPort)
	}()

	// Периодическая проверка лицензии: при отзыве останавливаем раннеры и viewerbot
	if !skipLicense && licenseServerURL != "" && gate != nil && licStore != nil && deviceFP != "" {
		go func() {
			ticker := time.NewTicker(2 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if !gate.Valid() {
						continue
					}
					if doLicenseValidate(ctx, licStore, licenseServerURL, deviceFP) {
						continue
					}
					gate.SetValid(false)
					manager.Stop()
					vb.Stop()
					log.Println("License revoked or expired. Runners and viewerbot stopped. Activate a key again in the dashboard.")
				}
			}
		}()
	}

	accessToken, _, proxy, _ := store.Current()
	resolveChannel := func(token, proxyStr string) int {
		slug := getChannelSlug()
		if slug == "" {
			return 0
		}
		try := func(proxy string) (int, error) {
			client := apiClientWithProxy(proxy)
			api, err := kick.NewAPIClient(kickapitypes.APIClientConfig{HTTPClient: client})
			if err != nil {
				return 0, err
			}
			channels, err := api.Channel().GetChannelsByBroadcasterSlug(ctx, token, []string{slug})
			if err != nil {
				return 0, err
			}
			if len(channels.Data) == 0 {
				return 0, nil
			}
			return channels.Data[0].BroadcasterUserID, nil
		}
		id, err := try(proxyStr)
		if err != nil && proxyStr != "" && isNetworkOrProxyError(err) {
			id, _ = try("")
			if id != 0 {
				log.Printf("Proxy unreachable, using direct connection")
			}
		}
		return id
	}
	setBID(resolveChannel(accessToken, proxy))
	if getBID() == 0 {
		var currentNum int
		for _, a := range store.List() {
			if a.Current {
				currentNum = a.Num
				break
			}
		}
		if currentNum != 0 {
			if _, err := refreshFunc(currentNum); err == nil {
				accessToken, _, proxy, _ = store.Current()
				setBID(resolveChannel(accessToken, proxy))
				if getBID() != 0 {
					log.Printf("Token refreshed. Channel %s -> broadcaster_user_id=%d", getChannelSlug(), getBID())
				}
			}
		}
	}
	if getBID() != 0 {
		log.Printf("Channel %s -> broadcaster_user_id=%d", getChannelSlug(), getBID())
	} else {
		log.Println("Current account unauthorized (401) or channel not found. Pick an account in the dashboard or add one (Add account).")
	}

	_, currentName, _, _ := store.Current()
	if currentName != "" {
		log.Printf("Dashboard selected account: %s", currentName)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
}
