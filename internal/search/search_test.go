package search

import (
	"net"
	"net/url"
	"testing"
)

func TestBlockedHTTPIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1",
		"10.0.0.1",
		"172.16.0.1",
		"192.168.1.1",
		"169.254.169.254",
		"100.64.0.1",
		"::1",
		"fc00::1",
	}
	for _, raw := range blocked {
		if !blockedHTTPIP(net.ParseIP(raw)) {
			t.Fatalf("expected %s to be blocked", raw)
		}
	}
	if blockedHTTPIP(net.ParseIP("93.184.216.34")) {
		t.Fatal("expected public IPv4 address to be allowed")
	}
}

func TestValidatePublicHTTPURLRejectsLocalhost(t *testing.T) {
	u, err := url.Parse("http://localhost/test")
	if err != nil {
		t.Fatal(err)
	}
	if err := validatePublicHTTPURL(u); err == nil {
		t.Fatal("expected localhost URL to be rejected")
	}
}
