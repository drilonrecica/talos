// SPDX-License-Identifier: AGPL-3.0-only
package api

import (
	"context"
	"net/http"
	"time"

	"github.com/drilonrecica/binnacle/internal/coolify"
)

func (s *Server) EnableCoolify(integration *coolify.Integration, sessions Authorizer, csrf CSRFValidator) {
	guard := func(w http.ResponseWriter, r *http.Request) bool {
		if sessions == nil || !sessions.Authorize(r) {
			WriteError(w, 401, Error{Code: "unauthorized", Message: "A browser session is required."})
			return false
		}
		return true
	}
	s.Handle("/api/v1/integrations/coolify", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		switch r.Method {
		case http.MethodGet:
			WriteJSON(w, 200, integration.Status())
		case http.MethodPut:
			if csrf == nil || !csrf.ValidCSRF(r) {
				WriteError(w, 403, Error{Code: "csrf_invalid", Message: "A valid CSRF token is required."})
				return
			}
			var body struct{ URL, Token string }
			if DecodeJSON(r, &body) != nil {
				WriteError(w, 400, Error{Code: "invalid_request", Message: "Coolify configuration is invalid."})
				return
			}
			if err := integration.Configure(r.Context(), body.URL, body.Token); err != nil {
				WriteError(w, 400, Error{Code: "coolify_invalid", Message: err.Error()})
				return
			}
			WriteJSON(w, 200, integration.Status())
		default:
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only GET and PUT are supported."})
		}
	}))
	s.Handle("/api/v1/integrations/coolify/test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only POST is supported."})
			return
		}
		if csrf == nil || !csrf.ValidCSRF(r) {
			WriteError(w, 403, Error{Code: "csrf_invalid", Message: "A valid CSRF token is required."})
			return
		}
		var body struct{ URL, Token string }
		if DecodeJSON(r, &body) != nil {
			WriteError(w, 400, Error{Code: "invalid_request", Message: "Coolify configuration is invalid."})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if err := integration.Test(ctx, body.URL, body.Token); err != nil {
			WriteError(w, 502, Error{Code: "coolify_unavailable", Message: "Coolify connection test failed."})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
}
