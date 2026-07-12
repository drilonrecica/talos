// SPDX-License-Identifier: AGPL-3.0-only
package api

import (
	"fmt"
	"net/http"

	"github.com/drilonrecica/binnacle/internal/auth"
)

// EnableSetup intentionally exposes only the one-time claim flow. It is
// unavailable as soon as an admin has been created.
func (s *Server) EnableSetup(setup *auth.SetupService, protection *auth.Protection, sessions *auth.Sessions) {
	allow := func(w http.ResponseWriter, r *http.Request) bool {
		if protection == nil {
			return true
		}
		ok, retry := protection.AllowSetup(r)
		if ok {
			return true
		}
		seconds := maxRetry(retry)
		w.Header().Set("Retry-After", fmt.Sprintf("%d", seconds))
		WriteError(w, http.StatusTooManyRequests, Error{Code: "rate_limited", Message: "Too many setup attempts. Try again later.", Details: map[string]int{"retryAfterSeconds": seconds}})
		return false
	}
	s.Handle("/api/v1/setup", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only GET is supported."})
			return
		}
		WriteJSON(w, 200, map[string]bool{"available": setup.Available(r.Context())})
	}))
	s.Handle("/api/v1/setup/verify", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only POST is supported."})
			return
		}
		if !allow(w, r) {
			return
		}
		var body struct {
			Token string `json:"token"`
		}
		if DecodeJSON(r, &body) != nil || setup.Verify(r.Context(), body.Token) != nil {
			WriteError(w, 401, Error{Code: "setup_token_invalid", Message: "The setup token is invalid or expired."})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	s.Handle("/api/v1/setup/claim", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only POST is supported."})
			return
		}
		if !allow(w, r) {
			return
		}
		var body struct {
			Token    string `json:"token"`
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if DecodeJSON(r, &body) != nil {
			WriteError(w, 400, Error{Code: "invalid_request", Message: "Setup credentials are invalid."})
			return
		}
		user, err := setup.Claim(r.Context(), body.Token, body.Username, body.Password)
		if err != nil {
			WriteError(w, 400, Error{Code: "setup_claim_failed", Message: "The setup request could not be completed."})
			return
		}
		if sessions != nil {
			token, csrf, session, issueErr := sessions.IssueForRequest(r.Context(), user.ID, r, protection.Proxies())
			if issueErr != nil {
				WriteError(w, 500, Error{Code: "session_error", Message: "The administrator was created; sign in to continue."})
				return
			}
			secure := protection.Proxies().Secure(r)
			auth.SetSessionCookie(w, token, secure, session.AbsoluteExpires)
			auth.SetCSRFCookie(w, csrf, secure, session.AbsoluteExpires)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
}
