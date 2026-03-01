package main

import (
	"errors"
	"net"
	"strings"
)

// isNetworkOrProxyError returns true if err looks like a connection/timeout/proxy failure (so we can fallback to direct).
func isNetworkOrProxyError(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	s := strings.ToLower(err.Error())
	for _, phrase := range []string{"connection refused", "connection reset", "timeout", "proxy", "dial ", "no such host", "network is unreachable"} {
		if strings.Contains(s, phrase) {
			return true
		}
	}
	return false
}
