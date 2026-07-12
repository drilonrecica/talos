// SPDX-License-Identifier: AGPL-3.0-only
package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/drilonrecica/binnacle/internal/storage"
)

func TestSessionLifecycleAndHashedPersistence(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	manager := storage.New(filepath.Join(dir, "binnacle.db"), filepath.Join(dir, "run"))
	if err := manager.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	sessions := NewSessions(manager.DB(), SessionConfig{IdleTimeout: time.Hour, AbsoluteLifetime: 24 * time.Hour})
	sessions.now = func() time.Time { return now }
	r := httptest.NewRequest("POST", "https://binnacle.test/api/v1/auth/login", nil)
	r.RemoteAddr = "192.0.2.10:1234"
	r.Header.Set("User-Agent", "test-browser")
	token, csrf, session, err := sessions.IssueForRequest(ctx, "usr_test", r, TrustedProxies{})
	if err != nil {
		t.Fatal(err)
	}
	if token == "" || csrf == "" || !session.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatal("invalid issued session")
	}
	mutation := httptest.NewRequest("POST", "https://binnacle.test/api/v1/auth/logout", nil)
	mutation.Header.Set("Origin", "https://binnacle.test")
	mutation.Header.Set("X-CSRF-Token", csrf)
	mutation.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
	mutation.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: csrf})
	if !sessions.ValidCSRF(mutation) {
		t.Fatal("valid CSRF rejected")
	}
	mutation.Header.Set("Origin", "https://evil.test")
	if sessions.ValidCSRF(mutation) {
		t.Fatal("cross-origin CSRF accepted")
	}
	var stored, ua, ip string
	if err = manager.DB().QueryRowContext(ctx, "SELECT id_hash,user_agent_hash,ip_prefix_hash FROM sessions").Scan(&stored, &ua, &ip); err != nil {
		t.Fatal(err)
	}
	if stored == token || ua == "" || ip == "" {
		t.Fatal("plaintext token or missing fingerprints")
	}
	now = now.Add(30 * time.Minute)
	if _, err = sessions.Authenticate(ctx, token); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Hour)
	if _, err = sessions.Authenticate(ctx, token); err == nil {
		t.Fatal("idle-expired session accepted")
	}
	now = now.Add(time.Second)
	removed, err := sessions.Cleanup(ctx, 500)
	if err != nil || removed != 1 {
		t.Fatalf("cleanup removed=%d err=%v", removed, err)
	}
}

func TestConcurrentSessionsAndLogoutAll(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	manager := storage.New(filepath.Join(dir, "binnacle.db"), filepath.Join(dir, "run"))
	if err := manager.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	sessions := NewSessions(manager.DB(), SessionConfig{IdleTimeout: time.Hour, AbsoluteLifetime: 24 * time.Hour})
	var wg sync.WaitGroup
	tokens := make(chan string, 8)
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, _, err := sessions.Issue(ctx, "usr_test")
			if err != nil {
				errs <- err
				return
			}
			tokens <- token
		}()
	}
	wg.Wait()
	close(tokens)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if err := sessions.RevokeAll(ctx, "usr_test"); err != nil {
		t.Fatal(err)
	}
	for token := range tokens {
		if _, err := sessions.Authenticate(ctx, token); err == nil {
			t.Fatal("revoked session accepted")
		}
	}
}

func TestSessionCookieFlags(t *testing.T) {
	w := httptest.NewRecorder()
	SetSessionCookie(w, "token", true, time.Now().Add(time.Hour))
	cookie := w.Result().Cookies()[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != 2 || cookie.Path != "/" {
		t.Fatalf("cookie=%+v", cookie)
	}
}

func TestCurrentSessionRevocation(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	manager := storage.New(filepath.Join(dir, "binnacle.db"), filepath.Join(dir, "run"))
	if err := manager.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	sessions := NewSessions(manager.DB(), SessionConfig{IdleTimeout: time.Hour, AbsoluteLifetime: 24 * time.Hour})
	token, _, err := sessions.Issue(ctx, "usr_test")
	if err != nil {
		t.Fatal(err)
	}
	if err = sessions.Revoke(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err = sessions.Authenticate(ctx, token); err == nil {
		t.Fatal("revoked current session accepted")
	}
}
