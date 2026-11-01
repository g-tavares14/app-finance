package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"app-finance/internal/usecases"
)

func TestClientIP(t *testing.T) {
	cases := []struct {
		name       string
		trustProxy bool
		remoteAddr string
		forwarded  string
		want       string
		wantOK     bool
	}{
		{"public ipv4", false, "8.8.8.8:1234", "", "8.8.8.8", true},
		{"loopback ignored", false, "127.0.0.1:1", "", "", false},
		{"docker gateway ignored", false, "172.18.0.1:5", "", "", false},
		{"ipv6 loopback ignored", false, "[::1]:9", "", "", false},
		{"ipv6 collapses to /64", false, "[2001:db8:1:2:aaaa:bbbb:cccc:dddd]:80", "", "2001:db8:1:2::/64", true},
		{"forwarded header ignored by default", false, "172.18.0.1:5", "1.2.3.4", "", false},
		{"proxy appended entry is used", true, "172.18.0.1:5", "9.9.9.9, 1.2.3.4", "1.2.3.4", true},
		{"private forwarded entry ignored", true, "172.18.0.1:5", "10.0.0.7", "", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := &Handlers{trustProxy: c.trustProxy}
			r := httptest.NewRequest("POST", "/login", nil)
			r.RemoteAddr = c.remoteAddr
			if c.forwarded != "" {
				r.Header.Set("X-Forwarded-For", c.forwarded)
			}

			got, ok := h.clientIP(r)
			if got != c.want || ok != c.wantOK {
				t.Fatalf("clientIP = %q, %v; want %q, %v", got, ok, c.want, c.wantOK)
			}
		})
	}
}

func TestLoginKeys(t *testing.T) {
	h := NewHandlers(nil, false)

	public := httptest.NewRequest("POST", "/login", nil)
	public.RemoteAddr = "8.8.8.8:1"
	keys := h.loginKeys(public, "  ANA@X.com ")
	if len(keys) != 3 {
		t.Fatalf("public keys = %d, want loose email, email+ip and ip", len(keys))
	}
	if keys[0].limiter != h.emailLimiter || keys[0].key != "email:ana@x.com" {
		t.Fatalf("first key = %+v, want loose normalized email key", keys[0])
	}
	if keys[1].key != "email+ip:ana@x.com|8.8.8.8" || keys[2].key != "ip:8.8.8.8" {
		t.Fatalf("keys = %+v", keys)
	}

	local := httptest.NewRequest("POST", "/login", nil)
	local.RemoteAddr = "127.0.0.1:1"
	keys = h.loginKeys(local, "ana@x.com")
	if len(keys) != 1 || keys[0].limiter != h.authLimiter || keys[0].key != "email:ana@x.com" {
		t.Fatalf("local keys = %+v, want only the strict email key", keys)
	}

	keys = h.loginKeys(public, strings.Repeat("a", 5000))
	if len(keys[0].key) > len("email:")+usecases.MaxEmailLength {
		t.Fatalf("email key has %d bytes, want it truncated", len(keys[0].key))
	}
}
