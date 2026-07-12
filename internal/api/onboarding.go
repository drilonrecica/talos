// SPDX-License-Identifier: AGPL-3.0-only
package api

import (
	"net/http"

	"github.com/drilonrecica/binnacle/internal/onboarding"
)

func (s *Server) EnableOnboarding(service *onboarding.Service, authorizer Authorizer, csrf CSRFValidator) {
	guard := func(w http.ResponseWriter, r *http.Request, mutation bool) bool {
		if authorizer == nil || !authorizer.Authorize(r) {
			WriteError(w, 401, Error{Code: "unauthorized", Message: "Authentication is required."})
			return false
		}
		if mutation && (csrf == nil || !csrf.ValidCSRF(r)) {
			WriteError(w, 403, Error{Code: "csrf_invalid", Message: "A valid CSRF token is required."})
			return false
		}
		return true
	}
	s.Handle("/api/v1/onboarding", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r, r.Method != http.MethodGet) {
			return
		}
		switch r.Method {
		case http.MethodGet:
			state, err := service.State(r.Context())
			if err != nil {
				WriteError(w, 500, Error{Code: "onboarding_error", Message: "Onboarding state is unavailable."})
				return
			}
			WriteJSON(w, 200, state)
		case http.MethodPatch:
			var body struct {
				ExposureMode    string `json:"exposureMode"`
				RetentionPreset string `json:"retentionPreset"`
			}
			if DecodeJSON(r, &body) != nil {
				WriteError(w, 400, Error{Code: "invalid_request", Message: "Onboarding choices are invalid."})
				return
			}
			state, err := service.Update(r.Context(), body.ExposureMode, body.RetentionPreset)
			if err != nil {
				WriteError(w, 400, Error{Code: "invalid_request", Message: "Onboarding choices are invalid."})
				return
			}
			WriteJSON(w, 200, state)
		default:
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only GET and PATCH are supported."})
		}
	}))
	s.Handle("/api/v1/onboarding/diagnostics", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !guard(w, r, true) {
			if r.Method != http.MethodPost {
				WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only POST is supported."})
			}
			return
		}
		var body struct {
			IncludeOutbound bool `json:"includeOutbound"`
		}
		if DecodeJSON(r, &body) != nil {
			WriteError(w, 400, Error{Code: "invalid_request", Message: "Diagnostics options are invalid."})
			return
		}
		state, err := service.Diagnose(r.Context(), body.IncludeOutbound)
		if err != nil {
			WriteError(w, 500, Error{Code: "diagnostics_error", Message: "Diagnostics could not be completed."})
			return
		}
		WriteJSON(w, 200, state)
	}))
	s.Handle("/api/v1/onboarding/complete", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !guard(w, r, true) {
			if r.Method != http.MethodPost {
				WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only POST is supported."})
			}
			return
		}
		state, err := service.Complete(r.Context())
		if err != nil {
			WriteError(w, 409, Error{Code: "onboarding_incomplete", Message: "Complete the required onboarding steps first."})
			return
		}
		WriteJSON(w, 200, state)
	}))
	s.Handle("/api/v1/onboarding/checklist/dismiss", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !guard(w, r, true) {
			if r.Method != http.MethodPost {
				WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only POST is supported."})
			}
			return
		}
		if err := service.DismissChecklist(r.Context()); err != nil {
			WriteError(w, 500, Error{Code: "onboarding_error", Message: "The checklist could not be dismissed."})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
}
