// Kick Chat — отправка сообщений в чат через OAuth (kick-go-sdk).
// Регистрация приложения: https://developers.kick.com/
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
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
	"github.com/henrikah/kick-go-sdk/v2/enums/kickscopes"
	"github.com/henrikah/kick-go-sdk/v2/kickapitypes"
	"github.com/henrikah/kick-go-sdk/v2/kickcontracts"
	"github.com/henrikah/kick-go-sdk/v2/kickoauthtypes"
	"github.com/joho/godotenv"
)

// Значения по умолчанию для лицензии — задаются при сборке через -ldflags.
// Юзеру не нужно прописывать LICENSE_SERVER_URL и LICENSE_HMAC_SECRET в .env.
var (
	defaultLicenseServerURL string
	defaultLicenseHMACSecret string
)

func apiClientWithProxy(proxyStr string) *http.Client {
	return httpFactory.Get(proxyStr)
}

// statusCodeFromError пытается извлечь HTTP-код из текста ошибки (429, 401, 5xx).
func statusCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	s := err.Error()
	for _, code := range []int{429, 401, 500, 502, 503, 504} {
		if strings.Contains(s, strconv.Itoa(code)) {
			return code
		}
	}
	return 0
}

var httpFactory *httpclient.Factory

const redirectPath = "/callback"

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
			return runner.SendResult{Err: err, StatusCode: statusCodeFromError(err)}
		}
		runner.RecordSendSuccess(latency)
		return runner.SendResult{}
	}
}

func main() {
	_ = godotenv.Load()
	// Логи в файл (не в консоль)
	logPath := os.Getenv("LOG_FILE")
	if logPath == "" {
		logPath = "kick-chat.log"
	}
	if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		log.SetOutput(io.MultiWriter(f, os.Stderr))
		log.SetFlags(log.Ldate | log.Ltime)
	}
	// при ошибке открытия файла логи остаются в stderr
	clientID := os.Getenv("KICK_CLIENT_ID")
	clientSecret := os.Getenv("KICK_CLIENT_SECRET")
	redirectURI := os.Getenv("KICK_REDIRECT_URI")
	channelSlug := os.Getenv("CHANNEL_SLUG")
	if channelSlug == "" {
		channelSlug = "mlaffonxd"
	}
	if clientID == "" || clientSecret == "" {
		log.Fatal("Set KICK_CLIENT_ID and KICK_CLIENT_SECRET (register app at https://developers.kick.com/)")
	}
	if redirectURI == "" {
		redirectURI = "http://localhost:8765/callback"
	}

	ctx := context.Background()
	oauthClient, err := kick.NewOAuthClient(kickoauthtypes.OAuthClientConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		HTTPClient:   http.DefaultClient,
	})
	if err != nil {
		log.Fatalf("OAuth client: %v", err)
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
	if store.Count() == 0 {
		accessToken, refreshToken, err := runOAuthFlow(ctx, oauthClient, redirectURI)
		if err != nil {
			log.Fatalf("OAuth: %v", err)
		}
		store.Add("", accessToken, refreshToken)
		if err := store.Save(); err != nil {
			log.Printf("Warning: could not save accounts: %v", err)
		}
		log.Println("Account 1 added. Accounts saved to", accountsFile)
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
			} else if time.Since(payload.LastValidationAt) < 7*24*time.Hour {
				gate.SetValid(true)
			} else {
				// Refresh
				refreshBody, _ := json.Marshal(map[string]string{
					"refresh_token": payload.RefreshToken,
					"device_id":     payload.DeviceID,
				})
				req, err := http.NewRequestWithContext(ctx, http.MethodPost, licenseServerURL+"/license/refresh", strings.NewReader(string(refreshBody)))
				if err == nil {
					req.Header.Set("Content-Type", "application/json")
					resp, err := http.DefaultClient.Do(req)
					if err == nil {
						var ref struct {
							SignedLicense string `json:"signed_license"`
							RefreshToken  string `json:"refresh_token"`
						}
						_ = json.NewDecoder(resp.Body).Decode(&ref)
						resp.Body.Close()
						if resp.StatusCode == http.StatusOK && ref.SignedLicense != "" {
							newRefresh := ref.RefreshToken
							if newRefresh == "" {
								newRefresh = payload.RefreshToken
							}
							if err := licStore.Save(&licensestore.Payload{
								SignedLicense:    ref.SignedLicense,
								RefreshToken:     newRefresh,
								DeviceID:         payload.DeviceID,
								LastValidationAt: time.Now().UTC(),
							}); err == nil {
								gate.SetValid(true)
							}
						}
					}
				}
				if !gate.Valid() {
					log.Println("License refresh failed or expired; re-activate in dashboard")
				}
			}
		}
	} else {
		// Обхода нет: без LICENSE_SERVER_URL доступ закрыт, показываем экран ввода ключа
		gate = &licenseGate{}
		// gate.Valid() остаётся false — дашборд и API заблокированы
	}
	vb := viewerbot.New()
	go runWebServer(ctx, store, channelSlug, dashboardPort, manager, vb, oauthClient, clientID, clientSecret, func() {
		time.Sleep(200 * time.Millisecond)
		os.Exit(0)
	}, gate, licStore, licenseServerURL, licenseHMACSecret, deviceFP)

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
				log.Printf("Прокси недоступен, использовано обычное соединение")
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
					log.Printf("Токен обновлён по refresh. Channel %s -> broadcaster_user_id=%d", channelSlug, broadcasterUserID)
				}
			}
		}
	}
	if broadcasterUserID != 0 {
		log.Printf("Channel %s -> broadcaster_user_id=%d", channelSlug, broadcasterUserID)
	} else {
		log.Println("Текущий аккаунт не авторизован (401) или канал не найден. Выберите аккаунт в дашборде или добавьте: add")
	}

	_, currentName, _, _ := store.Current()
	if currentName != "" {
		log.Printf("В дашборде выбран аккаунт: %s", currentName)
	}
	fmt.Println("Команды: add — добавить аккаунт, quit — выход. Выбор аккаунта — в дашборде.")
	scanner := bufio.NewScanner(os.Stdin)
loop:
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		// Выход разрешён без лицензии; остальное — только с валидной лицензией
		if lower != "!quit" && lower != "quit" && (licenseServerURL == "" || gate == nil || !gate.Valid()) {
			fmt.Println("Требуется лицензия. Откройте http://localhost:" + dashboardPort + " в браузере и введите ключ.")
			continue
		}
		switch {
		case lower == "!quit" || lower == "quit":
			break loop
		case lower == "add":
			accessToken, refreshToken, err := runOAuthFlow(ctx, oauthClient, redirectURI)
			if err != nil {
				log.Printf("OAuth failed: %v", err)
				continue
			}
			idx, _ := store.Add("", accessToken, refreshToken)
			if err := store.Save(); err != nil {
				log.Printf("Warning: could not save: %v", err)
			}
			_, name, _, _ := store.Current()
			log.Printf("Added account %d: %s", idx, name)
			tok, _, prox, _ := store.Current()
			if id := resolveChannel(tok, prox); id != 0 {
				broadcasterUserID = id
				log.Printf("Channel %s -> broadcaster_user_id=%d", channelSlug, broadcasterUserID)
			}
			continue
		}
		accessToken, _, proxy, _ := store.Current()
		if broadcasterUserID == 0 {
			broadcasterUserID = resolveChannel(accessToken, proxy)
			if broadcasterUserID == 0 {
				log.Println("Токен не подходит (401). Выберите аккаунт в дашборде или добавьте: add")
				continue
			}
			log.Printf("Channel %s -> broadcaster_user_id=%d", channelSlug, broadcasterUserID)
		}
		var currentNum int
		for _, a := range store.List() {
			if a.Current {
				currentNum = a.Num
				break
			}
		}
		if currentNum == 0 {
			log.Println("Нет выбранного аккаунта")
			continue
		}
		ok, reason := manager.Send(currentNum, line)
		if !ok {
			log.Printf("Send rejected: %s", reason)
			continue
		}
		log.Println("Queued.")
	}
	if err := scanner.Err(); err != nil {
		log.Printf("Read: %v", err)
	}
}

func runOAuthFlow(ctx context.Context, oauthClient kickcontracts.OAuthClient, redirectURI string) (accessToken, refreshToken string, err error) {
	scopes := kickscopes.Scopes{kickscopes.ChannelRead, kickscopes.ChatWrite}
	state := "kick-chat-go-state"
	data, err := oauthClient.InitiateAuthorization(redirectURI, state, scopes)
	if err != nil {
		return "", "", fmt.Errorf("initiate auth: %w", err)
	}

	fmt.Println("Ссылка для получения access token:")
	fmt.Println(data.AuthorizationURL)
	fmt.Println("Откройте в браузере, войдите в Kick и разрешите доступ.")
	fmt.Println()

	codeCh := make(chan string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc(redirectPath, func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", 400)
			return
		}
		codeCh <- code
		_, _ = w.Write([]byte("<p>OK. You can close this tab and return to the terminal.</p>"))
	})
	server := &http.Server{Addr: ":8765", Handler: mux}
	go func() {
		_ = server.ListenAndServe()
	}()
	defer func() { _ = server.Shutdown(context.Background()) }()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	var code string
	select {
	case code = <-codeCh:
		break
	case <-sigCh:
		return "", "", fmt.Errorf("cancelled")
	}

	tokenResp, err := oauthClient.ExchangeAuthorizationCode(ctx, redirectURI, code, data.PKCEVerifier)
	if err != nil {
		return "", "", fmt.Errorf("exchange code: %w", err)
	}
	return tokenResp.AccessToken, tokenResp.RefreshToken, nil
}
