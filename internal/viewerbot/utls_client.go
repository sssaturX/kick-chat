package viewerbot

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	utls "github.com/refraction-networking/utls"
)

func utlsDialTLS(ctx context.Context, network, addr, serverName string) (net.Conn, error) {
	if serverName == "" {
		serverName = hostFromAddr(addr)
	}
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	tcpConn, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}
	return utlsWrapConn(tcpConn, serverName)
}

// utlsWrapConn performs TLS handshake with Chrome fingerprint on an existing TCP conn.
func utlsWrapConn(tcpConn net.Conn, serverName string) (net.Conn, error) {
	if serverName == "" {
		serverName = tcpConn.RemoteAddr().String()
		if h, _, _ := net.SplitHostPort(serverName); h != "" {
			serverName = h
		}
	}
	config := &utls.Config{ServerName: serverName}
	uConn := utls.UClient(tcpConn, config, utls.HelloChrome_120)
	if err := uConn.Handshake(); err != nil {
		tcpConn.Close()
		return nil, err
	}
	return uConn, nil
}

// utlsHTTPClient returns an http.Client that uses Chrome TLS fingerprint (uTLS) for HTTPS.
func utlsHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, _, _ := net.SplitHostPort(addr)
				return utlsDialTLS(ctx, network, addr, host)
			},
		},
	}
}

func hostFromAddr(addr string) string {
	host, _, _ := net.SplitHostPort(addr)
	if host != "" {
		return host
	}
	return strings.Trim(addr, "[]")
}
