package viewerbot

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/net/publicsuffix"
	"golang.org/x/net/proxy"
)

const (
	clientToken     = "e1393935a959b4020a4491574f6490129f678acdaa92760471263db43487f823"
	wsHost          = "websockets.kick.com"
	wsPath          = "/viewer/v1/connect"
	tokenURL        = "https://websockets.kick.com/viewer/v1/token"
	channelAPIv2    = "https://kick.com/api/v2/channels/"
	pingIntervalMin = 12
	pingIntervalMax = 17
	reconnectDelay  = 3
	maxReconnect    = 30
	// pingsPerConnection matches Python kick-viewbot: send 10 pings then disconnect and reconnect.
	pingsPerConnection = 10
)

const channelHTTPTimeout = 30 * time.Second // longer for slow networks; Python tls_client has no short timeout

var defaultHTTP = &http.Client{Timeout: 15 * time.Second}

// kickHTTP returns HTTP client with Chrome TLS fingerprint (uTLS) to reduce 403.
func kickHTTP() *http.Client {
	return utlsHTTPClient()
}

// channelHTTP returns client for GetChannelID: same timeout as needed for slow Kick API, uTLS Chrome.
func channelHTTP() *http.Client {
	return &http.Client{
		Timeout: channelHTTPTimeout,
		Transport: &http.Transport{
			DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, _, _ := net.SplitHostPort(addr)
				return utlsDialTLS(ctx, network, addr, host)
			},
		},
	}
}

// kickAPIHeaders returns browser-like headers for Kick API requests (reduces 403).
func kickAPIHeaders(referer string) map[string]string {
	if referer == "" {
		referer = "https://kick.com/"
	}
	return map[string]string{
		"Accept":          "application/json, text/plain, */*",
		"Accept-Language": "en-US,en;q=0.9",
		"Accept-Encoding": "gzip, deflate, br",
		"Referer":         referer,
		"Origin":          "https://kick.com",
		"DNT":             "1",
		"Connection":      "keep-alive",
		"Sec-Fetch-Dest":  "empty",
		"Sec-Fetch-Mode":  "cors",
		"Sec-Fetch-Site":  "same-origin",
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"sec-ch-ua":       `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`,
		"sec-ch-ua-mobile": "?0",
		"sec-ch-ua-platform": `"Windows"`,
	}
}

// getToken fetches a one-time WebSocket token. Matches Python kick-viewbot: visit kick.com first (cookies), then token with X-CLIENT-TOKEN; fallback token URLs.
func getToken() (string, error) {
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	client := &http.Client{
		Timeout: 15 * time.Second,
		Jar:     jar,
		Transport: &http.Transport{
			DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, _, _ := net.SplitHostPort(addr)
				return utlsDialTLS(ctx, network, addr, host)
			},
		},
	}
	// 1) Visit kick.com first (like Python) to get cookies/session
	req, _ := http.NewRequest(http.MethodGet, "https://kick.com", nil)
	for k, v := range tokenDocHeaders() {
		req.Header.Set(k, v)
	}
	if resp, err := client.Do(req); err == nil {
		resp.Body.Close()
	}
	// 2) Token endpoints: main then fallbacks (same as Python)
	tokenEndpoints := []string{
		"https://websockets.kick.com/viewer/v1/token",
		"https://kick.com/api/websocket/token",
		"https://kick.com/api/v1/websocket/token",
	}
	for _, endpoint := range tokenEndpoints {
		req, _ = http.NewRequest(http.MethodGet, endpoint, nil)
		for k, v := range kickAPIHeaders("") {
			req.Header.Set(k, v)
		}
		req.Header.Set("X-CLIENT-TOKEN", clientToken)
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		var out struct {
			Data  struct { Token string `json:"token"` } `json:"data"`
			Token string `json:"token"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&out)
		resp.Body.Close()
		token := out.Data.Token
		if token == "" {
			token = out.Token
		}
		if token != "" {
			return token, nil
		}
	}
	return "", fmt.Errorf("token: no token from any endpoint")
}

// Bot runs viewer connections. Uses Python script test_view/kick-viewbot/kick.py when found; otherwise Go implementation.
type Bot struct {
	channelSlug string
	channelID   int64
	mu          sync.Mutex
	stop        chan struct{}
	done        chan struct{}
	running     bool
	target      int
	connected   int64
	startedAt   time.Time
	proxies     []string
	proxyIndex  uint32
	// Python runner
	pythonCmd        *exec.Cmd
	pythonStatusFile string
	pythonLog        *ringLog
}

const maxLogLines = 300

// ringLog is a ring buffer of lines for Python stdout/stderr, safe for concurrent use.
type ringLog struct {
	mu    sync.Mutex
	lines []string
}

func newRingLog() *ringLog { return &ringLog{lines: make([]string, 0, maxLogLines)} }

func (r *ringLog) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	sc := bufio.NewScanner(bytes.NewReader(p))
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r\n")
		r.lines = append(r.lines, line)
		if len(r.lines) > maxLogLines {
			r.lines = r.lines[1:]
		}
	}
	return len(p), nil
}

func (r *ringLog) Lines() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.lines))
	copy(out, r.lines)
	return out
}

// Status is the current viewer bot status for the API. When running Python script, extra fields come from its status file.
type Status struct {
	Running     bool   `json:"running"`
	Target      int    `json:"target"`
	Connected   int    `json:"connected"`
	StartedAt   string `json:"started_at,omitempty"`
	DurationSec int    `json:"duration_sec,omitempty"`
	// From Python script (when VIEWBOT_STATUS_FILE is used)
	Attempts   int `json:"attempts,omitempty"`
	Pings      int `json:"pings,omitempty"`
	Heartbeats int `json:"heartbeats,omitempty"`
	Viewers    int `json:"viewers,omitempty"` // stream viewer count from Kick
}

// New creates a new viewer bot (not started).
func New() *Bot {
	return &Bot{stop: make(chan struct{}), done: make(chan struct{})}
}

// pythonBin returns "python3" or "python" for the current OS. Used only when Python script is enabled (!release).
func pythonBin() string {
	if runtime.GOOS == "windows" {
		return "python"
	}
	return "python3"
}

// GetChannelID returns Kick channel ID by slug. Same as Python get_channel_info: one session, v2 then v1 then page scrape; no X-CLIENT-TOKEN on channel APIs.
func GetChannelID(slug string) (int64, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return 0, fmt.Errorf("empty channel slug")
	}
	slug = strings.ToLower(slug)
	if idx := strings.Index(slug, "kick.com/"); idx >= 0 {
		slug = strings.TrimPrefix(slug[idx+len("kick.com/"):], "/")
		if i := strings.IndexAny(slug, "/?"); i >= 0 {
			slug = slug[:i]
		}
	}
	headers := kickAPIHeaders("https://kick.com/")
	client := channelHTTP() // one client, 30s timeout, same as Python session

	// 1) v2 API — Python: no X-CLIENT-TOKEN for channel
	req, _ := http.NewRequest(http.MethodGet, channelAPIv2+url.PathEscape(slug), nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err == nil && resp.StatusCode == http.StatusOK {
		var data struct {
			ID int64 `json:"id"`
		}
		if json.NewDecoder(resp.Body).Decode(&data) == nil && data.ID != 0 {
			resp.Body.Close()
			return data.ID, nil
		}
		resp.Body.Close()
	}
	if resp != nil {
		resp.Body.Close()
	}

	// 2) v1 API
	req, _ = http.NewRequest(http.MethodGet, "https://kick.com/api/v1/channels/"+url.PathEscape(slug), nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Referer", "https://kick.com/"+slug)
	resp, err = client.Do(req)
	if err == nil && resp.StatusCode == http.StatusOK {
		var data struct {
			ID int64 `json:"id"`
		}
		if json.NewDecoder(resp.Body).Decode(&data) == nil && data.ID != 0 {
			resp.Body.Close()
			return data.ID, nil
		}
		resp.Body.Close()
	}
	if resp != nil {
		resp.Body.Close()
	}

	// 3) Scrape channel page — same patterns as Python
	req, _ = http.NewRequest(http.MethodGet, "https://kick.com/"+url.PathEscape(slug), nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("User-Agent", headers["User-Agent"])
	req.Header.Set("Referer", "https://kick.com/")
	resp, err = client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("channel: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("channel: v2/v1 failed and page returned %d", resp.StatusCode)
	}
	body := make([]byte, 0, 512*1024)
	for {
		b := make([]byte, 32*1024)
		n, _ := resp.Body.Read(b)
		if n == 0 {
			break
		}
		body = append(body, b[:n]...)
		if len(body) > 1024*1024 {
			break
		}
	}
	text := string(body)
	// Python order: "id":(\d+).*?"slug":"name", "channel_id":(\d+), channelId["']:\s*(\d+)
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`"id":\s*(\d+).*?"slug":\s*"` + regexp.QuoteMeta(slug) + `"`),
		regexp.MustCompile(`"channel_id":\s*(\d+)`),
		regexp.MustCompile(`channelId["']\s*:\s*(\d+)`),
	}
	for _, re := range patterns {
		m := re.FindStringSubmatch(text)
		if len(m) >= 2 {
			var id int64
			if _, err := fmt.Sscanf(m[1], "%d", &id); err == nil && id > 0 {
				return id, nil
			}
		}
	}
	return 0, fmt.Errorf("channel not found (no id in page)")
}

// tokenDocHeaders returns document/navigate headers like Python's get_token() first request.
func tokenDocHeaders() map[string]string {
	return map[string]string{
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		"Accept-Language":           "en-US,en;q=0.9",
		"Connection":                "keep-alive",
		"Sec-Fetch-Dest":            "document",
		"Sec-Fetch-Mode":            "navigate",
		"Sec-Fetch-Site":            "none",
		"Sec-Fetch-User":            "?1",
		"Upgrade-Insecure-Requests": "1",
		"User-Agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"sec-ch-ua":                 `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`,
		"sec-ch-ua-mobile":          "?0",
		"sec-ch-ua-platform":        `"Windows"`,
	}
}

// nextProxy returns next proxy string (round-robin); may be empty.
func (b *Bot) nextProxy() string {
	if len(b.proxies) == 0 {
		return ""
	}
	i := atomic.AddUint32(&b.proxyIndex, 1) - 1
	return b.proxies[i%uint32(len(b.proxies))]
}

// parseProxy returns (host, port, user, pass) for "host:port" or "host:port:user:pass".
func parseProxy(s string) (host, port, user, pass string, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", "", "", "", false
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 {
		return "", "", "", "", false
	}
	host = parts[0]
	port = parts[1]
	if len(parts) >= 4 {
		user, pass = parts[2], parts[3]
	}
	ok = true
	return
}

// runOne runs a single viewer connection; reconnects until stop.
func (b *Bot) runOne(ctx context.Context, id int) {
	defer func() {
		atomic.AddInt64(&b.connected, -1)
	}()

	for {
		select {
		case <-b.stop:
			return
		default:
		}

		token, err := getToken()
		if err != nil {
			log.Printf("[viewerbot] token error: %v", err)
			time.Sleep(time.Duration(reconnectDelay+rand.Intn(3)) * time.Second)
			continue
		}

		conn, err := b.dialWS(ctx, token)
		if err != nil {
			time.Sleep(time.Duration(reconnectDelay+rand.Intn(3)) * time.Second)
			continue
		}

		atomic.AddInt64(&b.connected, 1)

		b.pingLoop(conn)

		conn.Close()

		select {
		case <-b.stop:
			return
		default:
			time.Sleep(time.Duration(reconnectDelay+rand.Intn(5)) * time.Second)
		}
	}
}

func (b *Bot) dialWS(ctx context.Context, token string) (*websocket.Conn, error) {
	u := url.URL{Scheme: "wss", Host: wsHost, Path: wsPath, RawQuery: "token=" + url.QueryEscape(token)}
	dialer := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
		NetDialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			var tcpConn net.Conn
			var err error
			proxyStr := b.nextProxy()
			if proxyStr != "" {
				host, port, user, pass, ok := parseProxy(proxyStr)
				if ok {
					var auth *proxy.Auth
					if user != "" {
						auth = &proxy.Auth{User: user, Password: pass}
					}
					socksDialer, dialErr := proxy.SOCKS5(network, host+":"+port, auth, proxy.Direct)
					if dialErr != nil {
						return nil, dialErr
					}
					tcpConn, err = socksDialer.Dial(network, addr)
				}
			}
			if tcpConn == nil {
				tcpConn, err = (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, network, addr)
			}
			if err != nil {
				return nil, err
			}
			return utlsWrapConn(tcpConn, hostFromAddr(addr))
		},
	}
	conn, _, err := dialer.DialContext(ctx, u.String(), nil)
	return conn, err
}

func (b *Bot) pingLoop(conn *websocket.Conn) {
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var m struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(msg, &m) == nil && (m.Type == "pusher:ping" || m.Type == "ping") {
				pongType := "pusher:pong"
				if m.Type == "ping" {
					pongType = "pong"
				}
				_ = conn.WriteJSON(map[string]string{"type": pongType})
			}
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		}
	}()

	// channel_handshake first (same as Python)
	handshake := map[string]interface{}{
		"type": "channel_handshake",
		"data": map[string]interface{}{
			"message": map[string]interface{}{"channelId": b.channelID},
		},
	}
	if err := conn.WriteJSON(handshake); err != nil {
		return
	}

	// Exactly like Python: 10 pings with 12–17s between, then disconnect (runOne will reconnect).
	for pingCount := 0; pingCount < pingsPerConnection; pingCount++ {
		select {
		case <-b.stop:
			return
		default:
		}
		sleep := time.Duration(pingIntervalMin+rand.Intn(pingIntervalMax-pingIntervalMin+1)) * time.Second
		time.Sleep(sleep)
		select {
		case <-b.stop:
			return
		default:
		}
		if err := conn.WriteJSON(map[string]string{"type": "ping"}); err != nil {
			return
		}
	}
}

// Start starts the viewer bot. Prefers external runner: бинарник viewerbot (PyInstaller) или kick.py при разработке.
func (b *Bot) Start(channelSlug string, target int, proxies []string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.running {
		return fmt.Errorf("already running")
	}
	channelSlug = strings.TrimSpace(channelSlug)
	if channelSlug == "" {
		return fmt.Errorf("channel slug required")
	}
	if target <= 0 || target > 5000 {
		return fmt.Errorf("target viewers must be 1–5000")
	}

	runnerPath, isPython := resolveViewerbotRunner()
	if runnerPath != "" {
		return b.startExternal(runnerPath, isPython, channelSlug, target)
	}

	// Fallback: Go implementation
	cid, err := GetChannelID(channelSlug)
	if err != nil {
		return fmt.Errorf("channel: %w", err)
	}
	var list []string
	for _, p := range proxies {
		p = strings.TrimSpace(p)
		if p != "" {
			list = append(list, p)
		}
	}
	b.channelSlug = channelSlug
	b.channelID = cid
	b.target = target
	b.proxies = list
	b.proxyIndex = 0
	atomic.StoreInt64(&b.connected, 0)
	b.startedAt = time.Now()
	b.stop = make(chan struct{})
	b.running = true
	log.Printf("[viewerbot] starting %d viewers for channel %s (id=%d), proxies=%d", target, channelSlug, cid, len(list))
	ctx := context.Background()
	for i := 0; i < target; i++ {
		id := i
		go b.runOne(ctx, id)
		if (i+1)%50 == 0 {
			time.Sleep(200 * time.Millisecond)
		} else {
			time.Sleep(20 * time.Millisecond)
		}
	}
	return nil
}

// startExternal runs viewerbot: либо бинарник (viewerbot), либо python3 kick.py. Один и тот же CLI: --channel, --viewers, --quiet.
func (b *Bot) startExternal(runnerPath string, isPython bool, channelSlug string, target int) error {
	statusFile := filepath.Join(os.TempDir(), fmt.Sprintf("kick-viewbot-status-%d.json", os.Getpid()))
	args := []string{"--channel", channelSlug, "--viewers", strconv.Itoa(target), "--quiet"}
	var cmd *exec.Cmd
	if isPython {
		cmd = exec.Command(pythonBin(), append([]string{runnerPath}, args...)...)
		cmd.Dir = filepath.Dir(runnerPath)
	} else {
		cmd = exec.Command(runnerPath, args...)
		cmd.Dir = filepath.Dir(runnerPath)
	}
	b.pythonLog = newRingLog()
	w := io.MultiWriter(b.pythonLog, os.Stdout)
	cmd.Stdout = w
	cmd.Stderr = w
	cmd.Env = append(os.Environ(), "VIEWBOT_STATUS_FILE="+statusFile)
	if err := cmd.Start(); err != nil {
		if isPython {
			return fmt.Errorf("python viewbot: %w", err)
		}
		return fmt.Errorf("viewerbot binary: %w", err)
	}
	b.pythonCmd = cmd
	b.pythonStatusFile = statusFile
	b.channelSlug = channelSlug
	b.target = target
	b.startedAt = time.Now()
	b.running = true
	b.stop = make(chan struct{})
	kind := "binary"
	if isPython {
		kind = "Python script"
	}
	log.Printf("[viewerbot] started %s %s: channel=%s viewers=%d", kind, runnerPath, channelSlug, target)
	go func() {
		_ = cmd.Wait()
		b.mu.Lock()
		if b.pythonCmd == cmd {
			b.pythonCmd = nil
			b.pythonStatusFile = ""
			b.running = false
			log.Printf("[viewerbot] external viewerbot exited")
		}
		b.mu.Unlock()
	}()
	return nil
}

// Stop stops the bot (Python process or Go goroutines).
func (b *Bot) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.running {
		return
	}
	if b.pythonCmd != nil && b.pythonCmd.Process != nil {
		_ = b.pythonCmd.Process.Kill()
		b.pythonCmd = nil
	}
	close(b.stop)
	b.running = false
	log.Printf("[viewerbot] stop requested")
}

// GetStatus returns current status (safe to call from any goroutine). When Python is running, reads status from its file.
func (b *Bot) GetStatus() Status {
	b.mu.Lock()
	running := b.running
	target := b.target
	startedAt := b.startedAt
	statusFile := b.pythonStatusFile
	b.mu.Unlock()

	st := Status{Running: running, Target: target, StartedAt: startedAt.Format(time.RFC3339)}
	if statusFile != "" {
		if data, err := os.ReadFile(statusFile); err == nil {
			var py struct {
				Connections int `json:"connections"`
				Attempts    int `json:"attempts"`
				Pings       int `json:"pings"`
				Heartbeats  int `json:"heartbeats"`
				Viewers     int `json:"viewers"`
				DurationSec int `json:"duration_sec"`
			}
			if json.Unmarshal(data, &py) == nil {
				st.Connected = py.Connections
				st.Attempts = py.Attempts
				st.Pings = py.Pings
				st.Heartbeats = py.Heartbeats
				st.Viewers = py.Viewers
				st.DurationSec = py.DurationSec
			}
		}
	}
	if st.DurationSec == 0 && running && !startedAt.IsZero() {
		st.DurationSec = int(time.Since(startedAt).Seconds())
	}
	if st.Connected == 0 && running {
		st.Connected = int(atomic.LoadInt64(&b.connected))
	}
	return st
}

// GetLog returns the last lines of Python stdout/stderr (when Python is or was running). Safe to call from any goroutine.
func (b *Bot) GetLog() []string {
	b.mu.Lock()
	rl := b.pythonLog
	b.mu.Unlock()
	if rl == nil {
		return nil
	}
	return rl.Lines()
}