package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
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
	clientSecret       string
	oauthRedirectURI   string // Initiate + exchange — must match Redirect URL in Kick app
	dashboardBaseURL   string // e.g. http://localhost:8080 — redirects after OAuth land here
	licenseGate        *licenseGate
	licenseStore      *licensestore.Store
	licenseServerURL  string
	licenseHMACSecret string
	deviceFingerprint string

	// autosend: по таймеру отправляет заготовленные сообщения из messages.txt выбранным аккаунтом
	autosendMu          sync.Mutex
	autosendEnabled     bool
	autosendIntervalSec int
	autosendNextIndex   int
	autosendStopCh      chan struct{}
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
		Name    string `json:"name"` // custom label; empty = show "Account N" in UI
		Current bool   `json:"current"`
		Proxy   string `json:"proxy"`
	}
	var list []acc
	for _, a := range srv.store.List() {
		_, _, proxy, _, _ := srv.store.GetAccountByIndex(a.Num)
		list = append(list, acc{
			ID:      a.Num,
			Name:    srv.store.NameAt(a.Num),
			Current: a.Current,
			Proxy:   proxy,
		})
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

const messagesFileName = "messages.txt"

// readMessagesFile возвращает строки из messages.txt (по одной строке — одно сообщение), без пустых.
func readMessagesFile() []string {
	data, err := os.ReadFile(messagesFileName)
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

func (srv *webServer) handleAPIMessages(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	messages := readMessagesFile()
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(map[string]interface{}{"messages": messages})
}

func (srv *webServer) handleAPIAutosend(rw http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		srv.autosendMu.Lock()
		enabled := srv.autosendEnabled
		interval := srv.autosendIntervalSec
		srv.autosendMu.Unlock()
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]interface{}{
			"enabled":       enabled,
			"interval_sec":  interval,
		})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Enabled     bool `json:"enabled"`
		IntervalSec int  `json:"interval_sec"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(rw, "invalid json", http.StatusBadRequest)
		return
	}
	sec := body.IntervalSec
	if sec < 10 {
		sec = 10
	}
	if sec > 86400 {
		sec = 86400
	}

	srv.autosendMu.Lock()
	if srv.autosendStopCh != nil {
		close(srv.autosendStopCh)
		srv.autosendStopCh = nil
	}
	srv.autosendEnabled = body.Enabled
	srv.autosendIntervalSec = sec
	if body.Enabled {
		stopCh := make(chan struct{})
		srv.autosendStopCh = stopCh
		srv.autosendMu.Unlock()

		go func() {
			ticker := time.NewTicker(time.Duration(sec) * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-srv.ctx.Done():
					return
				case <-stopCh:
					return
				case <-ticker.C:
					srv.autosendMu.Lock()
					nextIdx := srv.autosendNextIndex
					srv.autosendMu.Unlock()

					messages := readMessagesFile()
					if len(messages) == 0 {
						continue
					}
					msg := messages[nextIdx%len(messages)]
					srv.autosendMu.Lock()
					srv.autosendNextIndex = nextIdx + 1
					srv.autosendMu.Unlock()

					list := srv.store.List()
					var currentID int
					for _, a := range list {
						if a.Current {
							currentID = a.Num
							break
						}
					}
					if currentID == 0 {
						continue
					}
					srv.manager.Send(currentID, msg)
				}
			}
		}()
	} else {
		srv.autosendMu.Unlock()
	}

	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(map[string]bool{"ok": true})
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

// revalidateLicense calls the license server /validate; if status is not "active", sets gate to invalid.
// So when the user refreshes the dashboard after admin revoke, they get the license page again.
func (srv *webServer) revalidateLicense() {
	if srv.licenseServerURL == "" || srv.licenseGate == nil || srv.licenseStore == nil {
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
		strings.TrimSpace(srv.channelSlug) != ""
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
		rw.Header().Set("Cache-Control", "no-store")
		if srv.licenseGate == nil || !srv.licenseGate.Valid() || srv.licenseServerURL == "" {
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
		if srv.licenseGate == nil || !srv.licenseGate.Valid() || srv.licenseServerURL == "" {
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

func runWebServer(ctx context.Context, store *accountsStore, channelSlug string, port string, manager *runner.AccountManager, vb *viewerbot.Bot, oauthClient kickcontracts.OAuthClient, clientID, clientSecret string, shutdownFn func(), gate *licenseGate, licStore *licensestore.Store, licenseURL, licenseSecret, deviceFP string) {
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
		channelSlug:       channelSlug,
		manager:           manager,
		viewerBot:         vb,
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
	}
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
	mux.HandleFunc("/api/accounts/", srv.requireLicense(srv.handleAPIAccountsSub))
	mux.HandleFunc("/api/status", srv.requireLicense(srv.handleAPIStatus))
	mux.HandleFunc("/api/runners", srv.requireLicense(srv.handleAPIRunners))
	mux.HandleFunc("/api/send", srv.requireLicense(srv.handleAPISend))
	mux.HandleFunc("/api/messages", srv.requireLicense(srv.handleAPIMessages))
	mux.HandleFunc("/api/autosend", srv.requireLicense(srv.handleAPIAutosend))
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
