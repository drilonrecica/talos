// SPDX-License-Identifier: AGPL-3.0-only
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/drilonrecica/talos/internal/auth"
	"github.com/drilonrecica/talos/internal/storage"
)

func TestLoginRotationAndLogoutControls(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	manager := storage.New(filepath.Join(dir, "talos.db"), filepath.Join(dir, "run"))
	if err := manager.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	credentials := auth.NewCredentials(manager.DB())
	user, err := credentials.CreateAdmin(ctx, "admin", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	sessions := auth.NewSessions(manager.DB(), auth.SessionConfig{IdleTimeout: time.Hour, AbsoluteLifetime: 24 * time.Hour})
	protection := auth.NewProtection(128, auth.TrustedProxies{})
	server := New()
	server.EnableAuth(credentials, sessions, protection)
	login := func(previous *http.Cookie) (*http.Cookie, *http.Cookie) {
		request := httptest.NewRequest(http.MethodPost, "http://talos.test/api/v1/auth/login", bytes.NewBufferString(`{"username":"admin","password":"correct horse battery staple"}`))
		request.RemoteAddr = "192.0.2.10:1234"
		request.Header.Set("Content-Type", "application/json")
		if previous != nil {
			request.AddCookie(previous)
		}
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusNoContent {
			t.Fatalf("login status=%d body=%s", response.Code, response.Body.String())
		}
		var session, csrf *http.Cookie
		for _, cookie := range response.Result().Cookies() {
			if cookie.Name == auth.SessionCookieName {
				session = cookie
			}
			if cookie.Name == auth.CSRFCookieName {
				csrf = cookie
			}
		}
		if session == nil || csrf == nil {
			t.Fatal("missing auth cookies")
		}
		if !session.HttpOnly || session.SameSite != http.SameSiteLaxMode || session.Expires.Before(time.Now().Add(23*time.Hour)) {
			t.Fatalf("session cookie=%+v", session)
		}
		if csrf.HttpOnly || csrf.SameSite != http.SameSiteLaxMode || csrf.Expires.Before(time.Now().Add(23*time.Hour)) {
			t.Fatalf("csrf cookie=%+v", csrf)
		}
		return session, csrf
	}
	first, _ := login(nil)
	second, secondCSRF := login(first)
	if _, err = sessions.Authenticate(ctx, first.Value); err == nil {
		t.Fatal("pre-login session was not rotated")
	}
	if _, err = sessions.Authenticate(ctx, second.Value); err != nil {
		t.Fatal(err)
	}
	current := httptest.NewRequest(http.MethodGet, "http://talos.test/api/v1/auth/session", nil)
	current.AddCookie(second)
	currentResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(currentResponse, current)
	if currentResponse.Code != http.StatusOK {
		t.Fatalf("current session status=%d body=%s", currentResponse.Code, currentResponse.Body.String())
	}
	var currentBody struct {
		User struct {
			Username string `json:"username"`
		} `json:"user"`
		ExpiresAt string `json:"expiresAt"`
	}
	if err = json.Unmarshal(currentResponse.Body.Bytes(), &currentBody); err != nil || currentBody.User.Username != "admin" || currentBody.ExpiresAt == "" {
		t.Fatalf("current session body=%s err=%v", currentResponse.Body.String(), err)
	}
	missing := httptest.NewRequest(http.MethodPost, "http://talos.test/api/v1/auth/logout", nil)
	missing.Header.Set("Origin", "http://talos.test")
	missing.AddCookie(second)
	denied := httptest.NewRecorder()
	server.Handler().ServeHTTP(denied, missing)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status=%d", denied.Code)
	}
	invalid := httptest.NewRequest(http.MethodPost, "http://talos.test/api/v1/auth/logout", nil)
	invalid.Header.Set("Origin", "http://talos.test")
	invalid.Header.Set("X-CSRF-Token", "invalid")
	invalid.AddCookie(second)
	invalid.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "invalid"})
	denied = httptest.NewRecorder()
	server.Handler().ServeHTTP(denied, invalid)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("invalid CSRF status=%d", denied.Code)
	}
	logout := httptest.NewRequest(http.MethodPost, "http://talos.test/api/v1/auth/logout", nil)
	logout.Header.Set("Origin", "http://talos.test")
	logout.Header.Set("X-CSRF-Token", secondCSRF.Value)
	logout.AddCookie(second)
	logout.AddCookie(secondCSRF)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, logout)
	if response.Code != http.StatusNoContent {
		t.Fatalf("logout status=%d", response.Code)
	}
	cleared := map[string]bool{}
	for _, cookie := range response.Result().Cookies() {
		if cookie.MaxAge < 0 {
			cleared[cookie.Name] = true
		}
	}
	if !cleared[auth.SessionCookieName] || !cleared[auth.CSRFCookieName] {
		t.Fatalf("cleared cookies=%v", cleared)
	}
	if _, err = sessions.Authenticate(ctx, second.Value); err == nil {
		t.Fatal("logged-out session accepted")
	}
	one, csrf, _, err := sessions.IssueWithCSRF(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	other, _, err := sessions.Issue(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	all := httptest.NewRequest(http.MethodPost, "http://talos.test/api/v1/auth/logout-all", nil)
	all.Header.Set("Origin", "http://talos.test")
	all.Header.Set("X-CSRF-Token", csrf)
	all.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: one})
	all.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: csrf})
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, all)
	if response.Code != http.StatusNoContent {
		t.Fatalf("logout-all status=%d body=%s", response.Code, response.Body.String())
	}
	if _, err = sessions.Authenticate(ctx, other); err == nil {
		t.Fatal("logout-all left concurrent session active")
	}
}
