// SPDX-License-Identifier: AGPL-3.0-only
package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/drilonrecica/binnacle/internal/auth"
	"github.com/drilonrecica/binnacle/internal/storage"
)

func TestExternalSessionBootstrapRecordsProvenance(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	manager := storage.New(filepath.Join(dir, "db"), filepath.Join(dir, "run"))
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
	forwarded, _ := auth.ParseTrustedProxies([]string{"10.0.0.2/32"})
	proxy, err := auth.NewProxyAuthenticator(auth.ProxyAuthConfig{Mode: auth.ProxyAuthOnly, ProxyCIDRs: []string{"10.0.0.2/32"}, IdentityHeader: "X-Auth-Subject", AllowedSubject: "subject-1"}, forwarded)
	if err != nil {
		t.Fatal(err)
	}
	server := New()
	server.EnableProxyAuth(proxy, credentials, sessions)
	request := httptest.NewRequest(http.MethodPost, "http://binnacle.test/api/v1/auth/external-session", nil)
	request.RemoteAddr = "10.0.0.2:1234"
	request.Header.Set("Origin", "http://binnacle.test")
	request.Header.Set("X-Auth-Subject", "subject-1")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var sessionCookie *http.Cookie
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == auth.SessionCookieName {
			sessionCookie = cookie
		}
	}
	if sessionCookie == nil {
		t.Fatal("session cookie missing")
	}
	session, err := sessions.Authenticate(ctx, sessionCookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	if session.UserID != user.ID || session.AuthMethod != "proxy" || session.AuthSubject != "subject-1" {
		t.Fatalf("session=%+v", session)
	}
}

func TestExternalSessionRejectsUntrustedPeerAndCrossOrigin(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	manager := storage.New(filepath.Join(dir, "db"), filepath.Join(dir, "run"))
	if err := manager.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	credentials := auth.NewCredentials(manager.DB())
	_, _ = credentials.CreateAdmin(ctx, "admin", "correct horse battery staple")
	sessions := auth.NewSessions(manager.DB(), auth.SessionConfig{IdleTimeout: time.Hour, AbsoluteLifetime: time.Hour})
	proxy, _ := auth.NewProxyAuthenticator(auth.ProxyAuthConfig{Mode: auth.ProxyAuthOnly, ProxyCIDRs: []string{"10.0.0.1/32"}, IdentityHeader: "X-User", AllowedSubject: "admin"}, auth.TrustedProxies{})
	server := New()
	server.EnableProxyAuth(proxy, credentials, sessions)
	for _, test := range []struct {
		remote, origin string
		want           int
	}{{"192.0.2.1:1", "http://binnacle.test", 401}, {"10.0.0.1:1", "https://evil.test", 403}} {
		request := httptest.NewRequest(http.MethodPost, "http://binnacle.test/api/v1/auth/external-session", nil)
		request.RemoteAddr = test.remote
		request.Header.Set("Origin", test.origin)
		request.Header.Set("X-User", "admin")
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != test.want {
			t.Errorf("remote=%s origin=%s status=%d want %d", test.remote, test.origin, response.Code, test.want)
		}
	}
}
