// SPDX-License-Identifier: AGPL-3.0-only
package api

import (
	"net/http"

	"github.com/drilonrecica/talos/internal/auth"
)

// EnableSetup intentionally exposes only the one-time claim flow. It is
// unavailable as soon as an admin has been created.
func (s *Server) EnableSetup(setup *auth.SetupService, protection *auth.Protection) {
	allow := func(w http.ResponseWriter, r *http.Request) bool {
		if protection == nil {
			return true
		}
		ok, retry := protection.AllowSetup(r)
		if ok {
			return true
		}
		w.Header().Set("Retry-After", "300")
		WriteError(w, http.StatusTooManyRequests, Error{Code: "rate_limited", Message: "Too many setup attempts. Try again later."})
		_ = retry
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
		if _, err := setup.Claim(r.Context(), body.Token, body.Username, body.Password); err != nil {
			WriteError(w, 400, Error{Code: "setup_claim_failed", Message: "The setup request could not be completed."})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
}
