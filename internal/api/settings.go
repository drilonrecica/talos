// SPDX-License-Identifier: AGPL-3.0-only
package api

import (
	"errors"
	"net/http"

	"github.com/drilonrecica/binnacle/internal/settings"
)

func (s *Server) EnableSettings(service *settings.Service, authorizer Authorizer, csrf CSRFValidator) {
	s.Handle("/api/v1/settings", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authorizer == nil || !authorizer.Authorize(r) {
			WriteError(w, 401, Error{Code: "unauthorized", Message: "Authentication is required."})
			return
		}
		switch r.Method {
		case http.MethodGet:
			snapshot, err := service.Snapshot(r.Context())
			if err != nil {
				WriteError(w, 500, Error{Code: "settings_unavailable", Message: "Settings are unavailable."})
				return
			}
			WriteJSON(w, 200, snapshot)
		case http.MethodPatch:
			if csrf == nil || !csrf.ValidCSRF(r) {
				WriteError(w, 403, Error{Code: "csrf_invalid", Message: "A valid CSRF token is required."})
				return
			}
			var body struct {
				Revision int64             `json:"revision"`
				Changes  map[string]string `json:"changes"`
			}
			if DecodeJSON(r, &body) != nil {
				WriteError(w, 400, Error{Code: "invalid_request", Message: "Settings changes are invalid."})
				return
			}
			actor := "admin"
			if provider, ok := authorizer.(ActorProvider); ok {
				if value, found := provider.Actor(r); found {
					actor = value
				}
			}
			snapshot, err := service.Patch(r.Context(), body.Revision, body.Changes, actor)
			if errors.Is(err, settings.ErrRevisionConflict) {
				WriteError(w, 409, Error{Code: "settings_conflict", Message: "Settings changed in another session. Reload and try again."})
				return
			}
			if err != nil {
				WriteError(w, 400, Error{Code: "settings_invalid", Message: "One or more settings values are invalid."})
				return
			}
			WriteJSON(w, 200, snapshot)
		default:
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only GET and PATCH are supported."})
		}
	}))
}
