// SPDX-License-Identifier: AGPL-3.0-only
package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/drilonrecica/binnacle/internal/auth"
	"github.com/drilonrecica/binnacle/internal/preferences"
	"github.com/drilonrecica/binnacle/internal/storage"
)

func TestPreferencesRequireSessionAndCSRF(t *testing.T) {
	store := storage.New(t.TempDir()+"/test.db", t.TempDir())
	if err := store.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.DB().Exec(`INSERT INTO users(id,username,password_hash,created_at,updated_at) VALUES('user-1','admin','hash',1,1)`); err != nil {
		t.Fatal(err)
	}
	sessions := auth.NewSessions(store.DB(), auth.SessionConfig{IdleTimeout: time.Hour, AbsoluteLifetime: 24 * time.Hour})
	token, csrf, _, err := sessions.IssueWithCSRF(context.Background(), "user-1")
	if err != nil {
		t.Fatal(err)
	}
	server := New()
	server.EnablePreferences(preferences.NewRepository(store.DB()), sessions)
	request := httptest.NewRequest(http.MethodPut, "http://binnacle.test/api/v1/preferences", strings.NewReader(`{"schemaVersion":1,"theme":"light","density":"compact","pinnedResources":[],"landingPage":"events","chartRange":"6h","updatedAt":"0001-01-01T00:00:00Z"}`))
	request.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	request.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: csrf})
	request.Header.Set("X-CSRF-Token", csrf)
	request.Header.Set("Origin", "http://binnacle.test")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	request = httptest.NewRequest(http.MethodGet, "/api/v1/preferences", nil)
	request.Header.Set("Authorization", "Bearer invalid")
	request.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("token accessed session-only preferences: %d", response.Code)
	}
}
