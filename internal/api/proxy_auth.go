// SPDX-License-Identifier: AGPL-3.0-only
package api

import (
	"net/http"
	"time"

	"github.com/drilonrecica/binnacle/internal/auth"
)

func (s *Server) EnableProxyAuth(proxy *auth.ProxyAuthenticator, credentials *auth.Credentials, sessions *auth.Sessions) {
	s.Handle("/api/v1/auth/methods", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only GET is supported."})
			return
		}
		_, available := proxy.Subject(r)
		WriteJSON(w, 200, map[string]any{"mode": proxy.Mode(), "local": proxy.AllowsLocal(), "proxy": proxy.AllowsProxy(), "proxyAvailable": available})
	}))
	s.Handle("/api/v1/auth/external-session", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only POST is supported."})
			return
		}
		if !proxy.SameOrigin(r) {
			WriteError(w, 403, Error{Code: "origin_invalid", Message: "A same-origin request is required."})
			return
		}
		subject, ok := proxy.Subject(r)
		if !ok {
			WriteError(w, 401, Error{Code: "external_auth_invalid", Message: "External authentication is unavailable."})
			return
		}
		user, err := credentials.Administrator(r.Context())
		if err != nil {
			WriteError(w, 409, Error{Code: "setup_required", Message: "Initial administrator setup is required."})
			return
		}
		if previous := auth.TokenFromRequest(r); previous != "" {
			_ = sessions.Revoke(r.Context(), previous)
		}
		token, csrf, session, err := sessions.IssueForProxyRequest(r.Context(), user.ID, subject, r, proxy.CookieProxies())
		if err != nil {
			WriteError(w, 500, Error{Code: "session_error", Message: "Could not start session."})
			return
		}
		secure := proxy.Secure(r)
		auth.SetSessionCookie(w, token, secure, session.AbsoluteExpires)
		auth.SetCSRFCookie(w, csrf, secure, session.AbsoluteExpires)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Expires", time.Unix(0, 0).UTC().Format(http.TimeFormat))
		w.WriteHeader(http.StatusNoContent)
	}))
}
