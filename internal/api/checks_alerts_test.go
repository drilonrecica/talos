// SPDX-License-Identifier: AGPL-3.0-only
package api

import (
	"github.com/drilonrecica/binnacle/internal/auth"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChecksAndAlertsRequireAuthentication(t *testing.T) {
	server := New()
	protection := auth.NewProtection(32, auth.TrustedProxies{})
	server.EnableChecks(nil, nil, DemoAuthorizer(false), nil, protection)
	server.EnableAlerts(nil, DemoAuthorizer(false), nil, protection)
	for _, path := range []string{"/api/v1/checks", "/api/v1/alerts", "/api/v1/alert-rules", "/api/v1/silences"} {
		req := httptest.NewRequest(http.MethodGet, "http://binnacle.test"+path, nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s status=%d", path, rec.Code)
		}
	}
}
func TestChecksMutationRequiresAuthenticationBeforeCSRF(t *testing.T) {
	server := New()
	server.EnableChecks(nil, nil, DemoAuthorizer(false), nil)
	req := httptest.NewRequest(http.MethodPost, "http://binnacle.test/api/v1/checks", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}
