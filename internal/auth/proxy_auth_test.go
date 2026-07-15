// SPDX-License-Identifier: AGPL-3.0-only
package auth

import (
	"net/http/httptest"
	"testing"
)

func TestProxyIdentityRejectsSpoofingAndRequiresExactSubject(t *testing.T) {
	proxy, err := NewProxyAuthenticator(ProxyAuthConfig{Mode: LocalAndProxyAuth, ProxyCIDRs: []string{"10.0.0.2/32"}, IdentityHeader: "X-Forwarded-User", AllowedSubject: "admin@example.test"}, TrustedProxies{})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("POST", "http://binnacle.test/api/v1/auth/external-session", nil)
	request.RemoteAddr = "192.0.2.1:1234"
	request.Header.Set("X-Forwarded-User", "admin@example.test")
	request.Header.Set("X-Forwarded-For", "10.0.0.2")
	if _, ok := proxy.Subject(request); ok {
		t.Fatal("untrusted peer spoofed proxy identity")
	}
	request.RemoteAddr = "10.0.0.2:1234"
	request.Header.Set("X-Forwarded-User", "Admin@example.test")
	if _, ok := proxy.Subject(request); ok {
		t.Fatal("non-exact subject accepted")
	}
	request.Header.Set("X-Forwarded-User", "admin@example.test")
	if subject, ok := proxy.Subject(request); !ok || subject != "admin@example.test" {
		t.Fatal("trusted exact subject rejected")
	}
	request.Header.Add("X-Forwarded-User", "admin@example.test")
	if _, ok := proxy.Subject(request); ok {
		t.Fatal("duplicate identity headers accepted")
	}
	request.Header.Set("X-Forwarded-User", "admin@example.test,attacker")
	if _, ok := proxy.Subject(request); ok {
		t.Fatal("appended identity header accepted")
	}
}

func TestLocalModeIgnoresIdentityHeaders(t *testing.T) {
	proxy, err := NewProxyAuthenticator(ProxyAuthConfig{Mode: LocalAuth}, TrustedProxies{})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("POST", "http://binnacle.test", nil)
	request.RemoteAddr = "127.0.0.1:1"
	request.Header.Set("X-Forwarded-User", "admin")
	if _, ok := proxy.Subject(request); ok {
		t.Fatal("local mode accepted external identity")
	}
}
