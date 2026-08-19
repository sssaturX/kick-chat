package httpclient

import "testing"

func TestParseProxy(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantHost string
		wantPort string
		wantUser string
		wantPass string
		wantOK   bool
	}{
		{name: "host port", input: "127.0.0.1:1080", wantHost: "127.0.0.1", wantPort: "1080", wantOK: true},
		{name: "host port auth", input: "127.0.0.1:1080:user:pass", wantHost: "127.0.0.1", wantPort: "1080", wantUser: "user", wantPass: "pass", wantOK: true},
		{name: "empty", input: "", wantOK: false},
		{name: "invalid", input: "127.0.0.1", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, user, pass, ok := parseProxy(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("ok=%v want %v", ok, tt.wantOK)
			}
			if host != tt.wantHost || port != tt.wantPort || user != tt.wantUser || pass != tt.wantPass {
				t.Fatalf("parseProxy(%q)=(%q,%q,%q,%q), want (%q,%q,%q,%q)", tt.input, host, port, user, pass, tt.wantHost, tt.wantPort, tt.wantUser, tt.wantPass)
			}
		})
	}
}
