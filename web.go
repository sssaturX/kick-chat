package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
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

// licenseGate guards dashboard and API when license server is configured.
type licenseGate struct {
	mu    sync.RWMutex
	valid bool
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
	broadcasterID     int
	broadcasterMu     sync.Mutex
	manager           *runner.AccountManager
	viewerBot         *viewerbot.Bot
	onShutdown        func()
	oauthClient       kickcontracts.OAuthClient
	clientID          string
	clientSecret      string
	webRedirectURI    string
	licenseGate       *licenseGate
	licenseStore      *licensestore.Store
	licenseServerURL  string
	licenseHMACSecret string
	deviceFingerprint string
}

func (w *webServer) resolveBroadcasterID(token, proxy string) int {
	client := apiClientWithProxy(proxy)
	api, err := kick.NewAPIClient(kickapitypes.APIClientConfig{HTTPClient: client})
	if err != nil {
		return 0
	}
	channels, err := api.Channel().GetChannelsByBroadcasterSlug(w.ctx, token, []string{w.channelSlug})
	if err != nil && proxy != "" && isNetworkOrProxyError(err) {
		api, _ = kick.NewAPIClient(kickapitypes.APIClientConfig{HTTPClient: http.DefaultClient})
		channels, err = api.Channel().GetChannelsByBroadcasterSlug(w.ctx, token, []string{w.channelSlug})
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
		ID      int    `json:"id"`
		Name    string `json:"name"`
		Current bool   `json:"current"`
		Proxy   string `json:"proxy"`
	}
	var list []acc
	for _, a := range srv.store.List() {
		_, _, proxy, _, _ := srv.store.GetAccountByIndex(a.Num)
		list = append(list, acc{ID: a.Num, Name: a.Name, Current: a.Current, Proxy: proxy})
	}
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(list)
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
	var list []st
	for i := 1; i <= srv.store.Count(); i++ {
		tok, _, proxy, _, ok := srv.store.GetAccountByIndex(i)
		if !ok {
			continue
		}
		list = append(list, st{ID: i, Online: srv.resolveBroadcasterID(tok, proxy) != 0})
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
		AccountID int    `json:"account_id"`
		Message   string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(rw, "invalid json", http.StatusBadRequest)
		return
	}
	if body.AccountID < 1 {
		http.Error(rw, "account_id required", http.StatusBadRequest)
		return
	}
	ok, reason := srv.manager.Send(body.AccountID, body.Message)
	if !ok {
		http.Error(rw, reason, http.StatusBadRequest)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(map[string]string{"status": "ok"})
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

func (srv *webServer) handleAPIChannel(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(map[string]string{"slug": srv.channelSlug})
}

func (srv *webServer) handleDashboard(rw http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/dashboard" && r.URL.Path != "/dashboard/" {
		http.NotFound(rw, r)
		return
	}
	// Требуем лицензию: либо сервер настроен и ключ не валиден, либо сервер не настроен (обхода нет)
	if srv.licenseGate != nil && !srv.licenseGate.Valid() || srv.licenseServerURL == "" {
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
		http.Error(rw, "OAuth not configured", http.StatusInternalServerError)
		return
	}
	state := oauthWebStatePrefix + fmt.Sprintf("%d", time.Now().UnixNano())
	data, err := srv.oauthClient.InitiateAuthorization(srv.webRedirectURI, state, kickscopes.Scopes{kickscopes.ChannelRead, kickscopes.ChatWrite})
	if err != nil {
		log.Printf("OAuth start: %v", err)
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	oauthPendingMu.Lock()
	oauthPending[state] = data.PKCEVerifier
	oauthPendingMu.Unlock()
	http.Redirect(rw, r, data.AuthorizationURL, http.StatusFound)
}

func (srv *webServer) handleOAuthCallback(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		http.Redirect(rw, r, "/?oauth=error", http.StatusFound)
		return
	}
	oauthPendingMu.Lock()
	verifier, ok := oauthPending[state]
	delete(oauthPending, state)
	oauthPendingMu.Unlock()
	if !ok {
		http.Redirect(rw, r, "/?oauth=expired", http.StatusFound)
		return
	}
	tokenResp, err := srv.oauthClient.ExchangeAuthorizationCode(srv.ctx, srv.webRedirectURI, code, verifier)
	if err != nil {
		log.Printf("OAuth exchange: %v", err)
		http.Redirect(rw, r, "/?oauth=exchange_failed", http.StatusFound)
		return
	}
	if _, err := srv.store.Add("", tokenResp.AccessToken, tokenResp.RefreshToken); err != nil {
		log.Printf("Add account: %v", err)
		http.Redirect(rw, r, "/?oauth=add_failed", http.StatusFound)
		return
	}
	if err := srv.store.Save(); err != nil {
		log.Printf("Save accounts: %v", err)
	}
	srv.manager.EnsureRunner(srv.store.Count())
	http.Redirect(rw, r, "/?added=1", http.StatusFound)
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
	log.Printf("Аккаунт переключён на #%d: %s", body.AccountID, name)
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(map[string]string{"status": "ok"})
}

func (srv *webServer) handleAPIAccountProxy(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/accounts/")
	if !strings.HasSuffix(path, "/proxy") {
		http.NotFound(rw, r)
		return
	}
	path = strings.TrimSuffix(path, "/proxy")
	path = strings.Trim(path, "/")
	id, err := strconv.Atoi(path)
	if err != nil || id < 1 {
		http.Error(rw, "invalid account id", http.StatusBadRequest)
		return
	}
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
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(map[string]string{"status": "ok"})
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
		slug = srv.channelSlug
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

// requireLicense returns 403 when license is not valid (в т.ч. если сервер не настроен — обхода нет).
func (srv *webServer) requireLicense(h http.HandlerFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		if srv.licenseGate == nil || !srv.licenseGate.Valid() || srv.licenseServerURL == "" {
			http.Error(rw, "license required", http.StatusForbidden)
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
	if srv.licenseServerURL == "" || srv.licenseGate == nil || srv.licenseStore == nil {
		http.Error(rw, "Сервер лицензий не настроен. Обратитесь к поставщику.", http.StatusServiceUnavailable)
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

func runWebServer(ctx context.Context, store *accountsStore, channelSlug string, port string, manager *runner.AccountManager, vb *viewerbot.Bot, oauthClient kickcontracts.OAuthClient, clientID, clientSecret string, shutdownFn func(), gate *licenseGate, licStore *licensestore.Store, licenseURL, licenseSecret, deviceFP string) {
	if port == "" {
		port = "8080"
	}
	webRedirectURI := "http://localhost:" + port + "/oauth/callback"
	srv := &webServer{
		ctx:               ctx,
		store:             store,
		channelSlug:       channelSlug,
		manager:           manager,
		viewerBot:         vb,
		onShutdown:        shutdownFn,
		oauthClient:       oauthClient,
		clientID:          clientID,
		clientSecret:      clientSecret,
		webRedirectURI:    webRedirectURI,
		licenseGate:       gate,
		licenseStore:      licStore,
		licenseServerURL:  licenseURL,
		licenseHMACSecret: licenseSecret,
		deviceFingerprint: deviceFP,
	}
	mux := http.NewServeMux()
	if emotesFS, err := fs.Sub(staticFS, "static/emotes"); err == nil {
		mux.Handle("/emotes/", http.StripPrefix("/emotes/", http.FileServer(http.FS(emotesFS))))
	}
	mux.HandleFunc("/", srv.handleDashboard)
	mux.HandleFunc("/license/activate", srv.handleLicenseActivate)
	mux.HandleFunc("/oauth/start", srv.requireLicense(srv.handleOAuthStart))
	mux.HandleFunc("/oauth/callback", srv.requireLicense(srv.handleOAuthCallback))
	mux.HandleFunc("/api/accounts", srv.requireLicense(srv.handleAPIAccounts))
	mux.HandleFunc("/api/accounts/current", srv.requireLicense(srv.handleAPICurrent))
	mux.HandleFunc("/api/accounts/", srv.requireLicense(srv.handleAPIAccountProxy))
	mux.HandleFunc("/api/status", srv.requireLicense(srv.handleAPIStatus))
	mux.HandleFunc("/api/runners", srv.requireLicense(srv.handleAPIRunners))
	mux.HandleFunc("/api/send", srv.requireLicense(srv.handleAPISend))
	mux.HandleFunc("/api/channel", srv.requireLicense(srv.handleAPIChannel))
	mux.HandleFunc("/api/shutdown", srv.requireLicense(srv.handleAPIShutdown))
	mux.HandleFunc("/api/viewerbot/start", srv.requireLicense(srv.handleViewerBotStart))
	mux.HandleFunc("/api/viewerbot/stop", srv.requireLicense(srv.handleViewerBotStop))
	mux.HandleFunc("/api/viewerbot/status", srv.requireLicense(srv.handleViewerBotStatus))
	mux.HandleFunc("/api/viewerbot/log", srv.requireLicense(srv.handleViewerBotLog))
	addr := ":" + port
	log.Printf("Dashboard: http://localhost%s", addr)
	_ = http.ListenAndServe(addr, mux)
}
