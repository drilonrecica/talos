// SPDX-License-Identifier: AGPL-3.0-only
package auth

import "testing"

func TestBinnacleCookieNames(t *testing.T) {
	if SessionCookieName != "binnacle_session" {
		t.Fatalf("session cookie name = %q", SessionCookieName)
	}
	if CSRFCookieName != "binnacle_csrf" {
		t.Fatalf("CSRF cookie name = %q", CSRFCookieName)
	}
}
