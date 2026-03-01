package httpclient

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

// Factory создаёт и кэширует HTTP-клиенты по ключу proxy. Один клиент на proxy string.
type Factory struct {
	mu      sync.Mutex
	clients map[string]*http.Client
}

// NewFactory возвращает фабрику с пустым кэшем.
func NewFactory() *Factory {
	return &Factory{clients: make(map[string]*http.Client)}
}

// Get возвращает *http.Client для proxyKey (пустая строка = без прокси). Клиенты кэшируются.
func (f *Factory) Get(proxyKey string) *http.Client {
	f.mu.Lock()
	defer f.mu.Unlock()
	if c := f.clients[proxyKey]; c != nil {
		return c
	}
	c := f.newClient(proxyKey)
	f.clients[proxyKey] = c
	return c
}

func (f *Factory) newClient(proxyKey string) *http.Client {
	transport := f.newTransport(proxyKey)
	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}
}

func (f *Factory) newTransport(proxyKey string) *http.Transport {
	// Требования ТЗ: централизованный Transport с keep-alive
	base := &http.Transport{
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:  10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:     false,
		ForceAttemptHTTP2:     true,
	}
	proxyKey = strings.TrimSpace(proxyKey)
	if proxyKey == "" {
		return base
	}
	dialCtx := f.dialContextForProxy(proxyKey)
	if dialCtx == nil {
		return base
	}
	base.DialContext = dialCtx
	return base
}

func parseProxy(s string) (host, port, user, pass string, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", "", "", "", false
	}
	parts := strings.SplitN(s, ":", 4)
	if len(parts) != 4 {
		return "", "", "", "", false
	}
	return parts[0], parts[1], parts[2], parts[3], true
}

func (f *Factory) dialContextForProxy(proxyStr string) func(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, user, pass, ok := parseProxy(proxyStr)
	if !ok {
		return nil
	}
	addr := net.JoinHostPort(host, port)
	auth := &proxy.Auth{User: user, Password: pass}
	dialer, err := proxy.SOCKS5("tcp", addr, auth, proxy.Direct)
	if err != nil {
		return nil
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialer.Dial(network, addr)
	}
}
