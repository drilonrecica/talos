// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authpkg "github.com/drilonrecica/binnacle/internal/auth"
)

func newSecurityServer(t *testing.T) *Server {
	t.Helper()
	p := authpkg.NewProtection(1024, authpkg.TrustedProxies{})
	s := New()
	s.SetLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.EnableLive(nil, DemoAuthorizer(true), p)
	return s
}

func TestSecurityHeaders(t *testing.T) {
	server := newSecurityServer(t)
	req := httptest.NewRequest(http.MethodGet, "http://binnacle.test/api/v1/session", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	headers := rec.Header()
	for _, key := range []string{"X-Request-ID", "X-Content-Type-Options", "X-Frame-Options", "Content-Security-Policy", "Referrer-Policy"} {
		if headers.Get(key) == "" {
			t.Errorf("missing security header: %s", key)
		}
	}
	if headers.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("unexpected X-Content-Type-Options value: %s", headers.Get("X-Content-Type-Options"))
	}
}

func TestHSTSOnTLS(t *testing.T) {
	server := newSecurityServer(t)
	req := httptest.NewRequest(http.MethodGet, "https://binnacle.test/api/v1/session", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if got := rec.Header().Get("Strict-Transport-Security"); got == "" {
		t.Error("missing HSTS header on HTTPS request")
	}
}

func TestUnauthorizedRequestRejected(t *testing.T) {
	server := New()
	p := authpkg.NewProtection(1024, authpkg.TrustedProxies{})
	server.EnableLive(nil, DemoAuthorizer(false), p)

	req := httptest.NewRequest(http.MethodGet, "http://binnacle.test/api/v1/live", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if rec.Header().Get("X-Frame-Options") == "" {
		t.Error("security headers missing on unauthorized response")
	}
}

func TestEventRangeCap(t *testing.T) {
	server := New()
	p := authpkg.NewProtection(1024, authpkg.TrustedProxies{})
	server.EnableEvents(nil, DemoAuthorizer(true), p)

	now := time.Now().UTC().Format(time.RFC3339)
	from := time.Now().UTC().Add(-10 * 24 * time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodGet, "http://binnacle.test/api/v1/events?from="+from+"&to="+now, nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for 10-day range, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "7 days") {
		t.Errorf("expected 7-day limit message, got %s", body)
	}
}

func TestMetricQueryMetricCountLimit(t *testing.T) {
	server := New()
	p := authpkg.NewProtection(1024, authpkg.TrustedProxies{})
	server.EnableMetrics(nil, DemoAuthorizer(true), p)

	now := time.Now().UTC().Format(time.RFC3339)
	from := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodGet, "http://binnacle.test/api/v1/metrics?scope=host&metrics=cpu,memory,network_rx,network_tx,block_read,block_write,cpu&from="+from+"&to="+now, nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for too many metrics, got %d", rec.Code)
	}
}

func TestRateLimitEvents(t *testing.T) {
	p := authpkg.NewProtection(1024, authpkg.TrustedProxies{})
	allowed := 0
	for i := 0; i < 70; i++ {
		req := httptest.NewRequest(http.MethodGet, "http://binnacle.test/", nil)
		req.RemoteAddr = "127.0.0.1:1234"
		ok, _ := p.AllowEvents(req)
		if ok {
			allowed++
		}
	}
	if allowed != 60 {
		t.Fatalf("expected 60 allowed event requests, got %d", allowed)
	}
}

func TestRequestLogRedactsSecrets(t *testing.T) {
	server := newSecurityServer(t)
	var buf bytes.Buffer
	server.SetLogger(slog.New(slog.NewJSONHandler(&buf, nil)))

	req := httptest.NewRequest(http.MethodGet, "http://binnacle.test/api/v1/session?token=super-secret&password=hunter2", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	log := buf.String()
	if strings.Contains(log, "super-secret") || strings.Contains(log, "hunter2") {
		t.Errorf("log leaked secret value: %s", log)
	}
	if !strings.Contains(log, "REDACTED") {
		t.Errorf("log did not mark redacted query value: %s", log)
	}
}

func TestOversizedBodyRejected(t *testing.T) {
	server := New()
	server.EnableAuth(nil, nil, authpkg.NewProtection(1024, authpkg.TrustedProxies{}))

	body := make([]byte, MaxRequestBodyBytes+1)
	req := httptest.NewRequest(http.MethodPost, "http://binnacle.test/api/v1/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized body, got %d", rec.Code)
	}
}
