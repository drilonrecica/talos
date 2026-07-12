// SPDX-License-Identifier: AGPL-3.0-only
package auth

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLimiterRecoversAfterRefill(t *testing.T) {
	now := time.Unix(0, 0)
	l := NewLimiter(2)
	l.now = func() time.Time { return now }
	p := BucketPolicy{Capacity: 1, Refill: time.Minute}
	if ok, _ := l.Allow("ip", p); !ok {
		t.Fatal("first request denied")
	}
	if ok, _ := l.Allow("ip", p); ok {
		t.Fatal("burst limit not enforced")
	}
	now = now.Add(time.Minute)
	if ok, _ := l.Allow("ip", p); !ok {
		t.Fatal("bucket did not refill")
	}
}

func TestProtectionPoliciesAreBounded(t *testing.T) {
	p := NewProtection(8, TrustedProxies{})
	for i := 0; i < 100; i++ {
		r := httptest.NewRequest("GET", "http://binnacle.test/api/v1/metrics", nil)
		r.RemoteAddr = fmt.Sprintf("198.18.%d.1:1234", i)
		_, _ = p.AllowMetrics(r)
	}
	if p.limiter.order.Len() > 8 {
		t.Fatalf("entries=%d", p.limiter.order.Len())
	}
	r := httptest.NewRequest("POST", "http://binnacle.test/api/v1/diagnostics", nil)
	r.RemoteAddr = "192.0.2.10:1234"
	for i := 0; i < 3; i++ {
		if ok, _ := p.AllowDiagnostics(r, "usr"); !ok {
			t.Fatal("diagnostics denied early")
		}
	}
	if ok, _ := p.AllowDiagnostics(r, "usr"); ok {
		t.Fatal("diagnostics burst accepted")
	}
}

func TestSameOriginRejectsSpoofing(t *testing.T) {
	r := httptest.NewRequest("POST", "https://binnacle.test/api/v1/auth/logout", nil)
	r.Host = "binnacle.test"
	r.Header.Set("Origin", "https://binnacle.test")
	if !SameOrigin(r, TrustedProxies{}) {
		t.Fatal("same origin rejected")
	}
	r.Header.Set("Origin", "https://evil.test")
	if SameOrigin(r, TrustedProxies{}) {
		t.Fatal("cross origin accepted")
	}
	r.Header.Del("Origin")
	if SameOrigin(r, TrustedProxies{}) {
		t.Fatal("missing origin accepted")
	}
}

func TestLoginAndSetupLimitsRecover(t *testing.T) {
	now := time.Unix(0, 0)
	p := NewProtection(32, TrustedProxies{})
	p.limiter.now = func() time.Time { return now }
	r := httptest.NewRequest("POST", "http://binnacle.test/api/v1/setup/verify", nil)
	r.RemoteAddr = "192.0.2.10:1234"
	for i := 0; i < 5; i++ {
		if ok, _ := p.AllowSetup(r); !ok {
			t.Fatal("setup denied early")
		}
	}
	if ok, _ := p.AllowSetup(r); ok {
		t.Fatal("setup burst accepted")
	}
	now = now.Add(5 * time.Minute)
	if ok, _ := p.AllowSetup(r); !ok {
		t.Fatal("setup did not recover")
	}
	for i := 0; i < 5; i++ {
		if ok, _ := p.AllowLogin(r, "admin"); !ok {
			t.Fatal("login denied early")
		}
	}
	if ok, _ := p.AllowLogin(r, "admin"); ok {
		t.Fatal("account burst accepted")
	}
}

func TestTrustedProxyStripsSpoofedForwardingEntries(t *testing.T) {
	proxies, err := ParseTrustedProxies([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "http://binnacle.test", nil)
	r.RemoteAddr = "10.0.0.1:1234"
	r.Header.Set("X-Forwarded-For", "203.0.113.9, 198.51.100.25, 10.0.0.2")
	if got := proxies.ClientPrefix(r); got != "198.51.100.0/24" {
		t.Fatalf("client prefix=%s", got)
	}
}
