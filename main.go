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
var (
	defaultLicenseServerURL  string
	defaultLicenseHMACSecret string
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

// Известные короткие коды официальных эмодзи Kick. API иногда отображает их как анимацию только без двоеточий.
var kickEmojiShortcodes = []string{
	"emojiCheerful", "emojiAngry", "emojiBlowKiss", "emojiAstonished", "emojiAngel",
	"emojiAwake", "emojiBubbly", "emojiClown", "emojiCool", "emojiCrave",
}

// normalizeKickEmojiContent заменяет :shortcode: на shortcode для известных эмодзи — так API Kick может распознать их как эмодзи.
func normalizeKickEmojiContent(content string) string {
	s := strings.TrimSpace(content)
	if len(s) < 3 || s[0] != ':' || s[len(s)-1] != ':' {
		return content
	}
	inner := s[1 : len(s)-1]
	for _, name := range kickEmojiShortcodes {
		if inner == name {
			return inner
		}
	}
	return content
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

// makeSendFunc возвращает runner.SendFunc. При 401 вызывает refreshFunc (если есть) и один раз повторяет отправку.
func makeSendFunc(store *accountsStore, factory *httpclient.Factory, ctx context.Context, channelSlug string, getBID func() int, refreshFunc func(accountID int) (newAccessToken string, err error)) runner.SendFunc {
	return func(accountID int, message string) runner.SendResult {
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
			return runner.SendResult{Err: errors.New("channel not resolved"), StatusCode: 503}
		}
		// Чтобы API Kick отображал эмодзи как анимации, отправляем shortcode без двоеточий (emojiCheerful вместо :emojiCheerful:).
		content := normalizeKickEmojiContent(message)
		// Короткий таймаут для отправки: при зависшем прокси не ждём 30 сек, быстро переходим на прямое соединение.
		sendCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		start := time.Now()
		_, err = api.Chat().SendChatMessageAsUser(sendCtx, tok, bid, nil, content)
		if err != nil && proxy != "" && isNetworkOrProxyError(err) {
			cancel()
			sendCtx, cancel = context.WithTimeout(ctx, 8*time.Second)
			api, _ = kick.NewAPIClient(kickapitypes.APIClientConfig{HTTPClient: factory.Get("")})
			if api != nil {
				_, err = api.Chat().SendChatMessageAsUser(sendCtx, tok, bid, nil, content)
			}
		}
		// При 401 пробуем обновить токен и повторить один раз
		if err != nil && statusCodeFromError(err) == 401 && refreshFunc != nil {
			newTok, refreshErr := refreshFunc(accountID)
			if refreshErr == nil && newTok != "" {
				tok = newTok
				cancel()
				sendCtx, cancel = context.WithTimeout(ctx, 8*time.Second)
				_, err = api.Chat().SendChatMessageAsUser(sendCtx, tok, bid, nil, content)
				if err != nil && proxy != "" && isNetworkOrProxyError(err) {
					api, _ = kick.NewAPIClient(kickapitypes.APIClientConfig{HTTPClient: factory.Get("")})
					if api != nil {
						_, err = api.Chat().SendChatMessageAsUser(sendCtx, tok, bid, nil, content)
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
	var broadcasterUserID int
	getBID := func() int { return broadcasterUserID }
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
	sendFunc := makeSendFunc(store, httpFactory, ctx, channelSlug, getBID, refreshFunc)
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
	skipLicense := os.Getenv("SKIP_LICENSE") == "1" || strings.ToLower(strings.TrimSpace(os.Getenv("SKIP_LICENSE"))) == "true"
	if skipLicense {
		gate.SetValid(true)
		log.Println("Test mode: SKIP_LICENSE=1 — license checks disabled")
	}
	// При старте без валидной лицензии не работаем: останавливаем раннеры (они уже созданы выше)
	if licenseServerURL != "" && gate != nil && !gate.Valid() && !skipLicense {
		manager.Stop()
	}
	vb := viewerbot.New()
	go runWebServer(ctx, store, channelSlug, dashboardPort, manager, vb, oauthClient, clientID, clientSecret, func() {
		time.Sleep(200 * time.Millisecond)
		os.Exit(0)
	}, gate, licStore, licenseServerURL, licenseHMACSecret, deviceFP)

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
		try := func(proxy string) (int, error) {
			client := apiClientWithProxy(proxy)
			api, err := kick.NewAPIClient(kickapitypes.APIClientConfig{HTTPClient: client})
			if err != nil {
				return 0, err
			}
			channels, err := api.Channel().GetChannelsByBroadcasterSlug(ctx, token, []string{channelSlug})
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
	broadcasterUserID = resolveChannel(accessToken, proxy)
	if broadcasterUserID == 0 {
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
				broadcasterUserID = resolveChannel(accessToken, proxy)
				if broadcasterUserID != 0 {
					log.Printf("Token refreshed. Channel %s -> broadcaster_user_id=%d", channelSlug, broadcasterUserID)
				}
			}
		}
	}
	if broadcasterUserID != 0 {
		log.Printf("Channel %s -> broadcaster_user_id=%d", channelSlug, broadcasterUserID)
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
