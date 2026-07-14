// SPDX-License-Identifier: AGPL-3.0-only
package api

import (
	"errors"
	"net/http"

	"github.com/drilonrecica/binnacle/internal/auth"
	"github.com/drilonrecica/binnacle/internal/preferences"
)

func (s *Server) EnablePreferences(repo *preferences.Repository, sessions *auth.Sessions) {
	s.Handle("/api/v1/preferences", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := sessions.Authenticate(r.Context(), auth.TokenFromRequest(r))
		if err != nil {
			WriteError(w, 401, Error{Code: "unauthorized", Message: "A browser session is required."})
			return
		}
		switch r.Method {
		case http.MethodGet:
			value, exists, err := repo.Get(r.Context(), session.UserID)
			if err != nil {
				WriteError(w, 500, Error{Code: "storage_error", Message: "Preferences are unavailable."})
				return
			}
			WriteJSON(w, 200, map[string]any{"exists": exists, "preferences": value})
		case http.MethodPut:
			if !sessions.ValidCSRF(r) {
				WriteError(w, 403, Error{Code: "csrf_invalid", Message: "A valid CSRF token is required."})
				return
			}
			var value preferences.Value
			if DecodeJSON(r, &value) != nil {
				WriteError(w, 400, Error{Code: "invalid_request", Message: "Preferences are invalid."})
				return
			}
			value, err = repo.Put(r.Context(), session.UserID, value)
			if errors.Is(err, preferences.ErrInvalid) {
				WriteError(w, 400, Error{Code: "preferences_invalid", Message: "Preferences are invalid."})
				return
			}
			if err != nil {
				WriteError(w, 500, Error{Code: "storage_error", Message: "Preferences could not be saved."})
				return
			}
			WriteJSON(w, 200, value)
		default:
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only GET and PUT are supported."})
		}
	}))
}
