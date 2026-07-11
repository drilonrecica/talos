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
		r := httptest.NewRequest("GET", "http://talos.test/api/v1/metrics", nil)
		r.RemoteAddr = fmt.Sprintf("198.18.%d.1:1234", i)
		_, _ = p.AllowMetrics(r)
	}
	if p.limiter.order.Len() > 8 {
		t.Fatalf("entries=%d", p.limiter.order.Len())
	}
	r := httptest.NewRequest("POST", "http://talos.test/api/v1/diagnostics", nil)
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
	r := httptest.NewRequest("POST", "https://talos.test/api/v1/auth/logout", nil)
	r.Host = "talos.test"
	r.Header.Set("Origin", "https://talos.test")
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
