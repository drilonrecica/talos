// SPDX-License-Identifier: AGPL-3.0-only
package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/drilonrecica/binnacle/internal/auth"
)

func (s *Server) EnableAuth(credentials *auth.Credentials, sessions *auth.Sessions, protection *auth.Protection, options ...any) {
	var mfa *auth.MFA
	var proxy *auth.ProxyAuthenticator
	for _, option := range options {
		switch value := option.(type) {
		case *auth.MFA:
			mfa = value
		case *auth.ProxyAuthenticator:
			proxy = value
		}
	}
	proxies := protection.Proxies()
	limited := func(w http.ResponseWriter, r *http.Request, username string) bool {
		ok, retry := protection.AllowLogin(r, username)
		if ok {
			return true
		}
		seconds := maxRetry(retry)
		w.Header().Set("Retry-After", fmt.Sprintf("%d", seconds))
		WriteError(w, 429, Error{Code: "rate_limited", Message: "Too many login attempts. Try again later.", Details: map[string]int{"retryAfterSeconds": seconds}})
		return false
	}
	s.Handle("/api/v1/auth/login", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only POST is supported."})
			return
		}
		if proxy != nil && !proxy.AllowsLocal() {
			WriteError(w, 404, Error{Code: "not_found", Message: "Local login is disabled."})
			return
		}
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Code     string `json:"code,omitempty"`
		}
		if DecodeJSON(r, &body) != nil {
			WriteError(w, http.StatusBadRequest, Error{Code: "invalid_request", Message: "A username and password are required."})
			return
		}
		if !limited(w, r, body.Username) {
			return
		}
		user, err := credentials.Authenticate(r.Context(), body.Username, body.Password)
		if err == nil && mfa != nil {
			err = mfa.Verify(r.Context(), user.ID, body.Code)
		}
		if err != nil {
			WriteError(w, 401, Error{Code: "invalid_credentials", Message: "Invalid username, password, or authentication code."})
			return
		}
		if previous := auth.TokenFromRequest(r); previous != "" {
			_ = sessions.Revoke(r.Context(), previous)
		}
		token, csrf, session, err := sessions.IssueForRequest(r.Context(), user.ID, r, proxies)
		if err != nil {
			WriteError(w, 500, Error{Code: "session_error", Message: "Could not start session."})
			return
		}
		secure := proxies.Secure(r)
		auth.SetSessionCookie(w, token, secure, session.AbsoluteExpires)
		auth.SetCSRFCookie(w, csrf, secure, session.AbsoluteExpires)
		w.WriteHeader(http.StatusNoContent)
	}))
	s.Handle("/api/v1/auth/session", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only GET is supported."})
			return
		}
		session, err := sessions.Authenticate(r.Context(), auth.TokenFromRequest(r))
		if err != nil {
			WriteError(w, 401, Error{Code: "unauthorized", Message: "Authentication is required."})
			return
		}
		user, err := credentials.UserByID(r.Context(), session.UserID)
		if err != nil {
			WriteError(w, 401, Error{Code: "unauthorized", Message: "Authentication is required."})
			return
		}
		WriteJSON(w, 200, map[string]any{
			"user":              map[string]string{"id": user.ID, "username": user.Username},
			"expiresAt":         session.ExpiresAt.UTC().Format(time.RFC3339),
			"absoluteExpiresAt": session.AbsoluteExpires.UTC().Format(time.RFC3339),
			"authMethod":        session.AuthMethod,
		})
	}))
	s.Handle("/api/v1/auth/logout", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only POST is supported."})
			return
		}
		if !sessions.ValidCSRF(r) {
			WriteError(w, 403, Error{Code: "csrf_invalid", Message: "A valid CSRF token is required."})
			return
		}
		_ = sessions.Revoke(r.Context(), auth.TokenFromRequest(r))
		auth.ClearSessionCookie(w, proxies.Secure(r))
		auth.ClearCSRFCookie(w, proxies.Secure(r))
		w.WriteHeader(http.StatusNoContent)
	}))
	s.Handle("/api/v1/auth/logout-all", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only POST is supported."})
			return
		}
		if !sessions.ValidCSRF(r) {
			WriteError(w, 403, Error{Code: "csrf_invalid", Message: "A valid CSRF token is required."})
			return
		}
		session, err := sessions.Authenticate(r.Context(), auth.TokenFromRequest(r))
		if err != nil {
			WriteError(w, 401, Error{Code: "unauthorized", Message: "Authentication is required."})
			return
		}
		_ = sessions.RevokeAll(r.Context(), session.UserID)
		auth.ClearSessionCookie(w, proxies.Secure(r))
		auth.ClearCSRFCookie(w, proxies.Secure(r))
		w.WriteHeader(http.StatusNoContent)
	}))
}

func (s *Server) EnableMFA(mfa *auth.MFA, credentials *auth.Credentials, sessions *auth.Sessions, protection *auth.Protection) {
	current := func(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
		session, err := sessions.Authenticate(r.Context(), auth.TokenFromRequest(r))
		if err != nil {
			WriteError(w, 401, Error{Code: "unauthorized", Message: "A browser session is required."})
			return auth.User{}, false
		}
		user, err := credentials.UserByID(r.Context(), session.UserID)
		if err != nil {
			WriteError(w, 401, Error{Code: "unauthorized", Message: "A browser session is required."})
			return auth.User{}, false
		}
		return user, true
	}
	csrf := func(w http.ResponseWriter, r *http.Request) bool {
		if !sessions.ValidCSRF(r) {
			WriteError(w, 403, Error{Code: "csrf_invalid", Message: "A valid CSRF token is required."})
			return false
		}
		return true
	}
	s.Handle("/api/v1/auth/mfa", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only GET is supported."})
			return
		}
		user, ok := current(w, r)
		if !ok {
			return
		}
		enabled, err := mfa.Enabled(r.Context(), user.ID)
		if err != nil {
			WriteError(w, 500, Error{Code: "mfa_unavailable", Message: "MFA status is unavailable."})
			return
		}
		WriteJSON(w, 200, map[string]bool{"enabled": enabled})
	}))
	s.Handle("/api/v1/auth/mfa/enroll", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only POST is supported."})
			return
		}
		user, ok := current(w, r)
		if !ok || !csrf(w, r) {
			return
		}
		if allowed, retry := protection.AllowLogin(r, user.Username); !allowed {
			seconds := maxRetry(retry)
			w.Header().Set("Retry-After", fmt.Sprint(seconds))
			WriteError(w, 429, Error{Code: "rate_limited", Message: "Too many authentication attempts."})
			return
		}
		var body struct {
			Password string `json:"password"`
		}
		if DecodeJSON(r, &body) != nil {
			WriteError(w, 400, Error{Code: "invalid_request", Message: "Current password is required."})
			return
		}
		enrollment, err := mfa.Begin(r.Context(), user, body.Password)
		if err != nil {
			WriteError(w, 400, Error{Code: "mfa_enrollment_failed", Message: "MFA enrollment could not be started."})
			return
		}
		WriteJSON(w, 200, enrollment)
	}))
	s.Handle("/api/v1/auth/mfa/confirm", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only POST is supported."})
			return
		}
		user, ok := current(w, r)
		if !ok || !csrf(w, r) {
			return
		}
		var body struct {
			Code string `json:"code"`
		}
		if DecodeJSON(r, &body) != nil {
			WriteError(w, 400, Error{Code: "invalid_request", Message: "Authentication code is required."})
			return
		}
		codes, err := mfa.Confirm(r.Context(), user.ID, body.Code)
		if err != nil {
			WriteError(w, 400, Error{Code: "mfa_confirmation_failed", Message: "Authentication code is invalid or expired."})
			return
		}
		WriteJSON(w, 200, map[string]any{"recoveryCodes": codes})
	}))
	s.Handle("/api/v1/auth/mfa/disable", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only POST is supported."})
			return
		}
		user, ok := current(w, r)
		if !ok || !csrf(w, r) {
			return
		}
		var body struct {
			Password string `json:"password"`
			Code     string `json:"code"`
		}
		if DecodeJSON(r, &body) != nil {
			WriteError(w, 400, Error{Code: "invalid_request", Message: "Password and authentication code are required."})
			return
		}
		if err := mfa.Disable(r.Context(), user, body.Password, body.Code); err != nil {
			WriteError(w, 401, Error{Code: "invalid_credentials", Message: "Invalid password or authentication code."})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
}
