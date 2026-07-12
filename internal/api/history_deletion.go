// SPDX-License-Identifier: AGPL-3.0-only
package api

import (
	"net/http"
	"strings"

	"github.com/drilonrecica/binnacle/internal/storage"
)

type CSRFValidator interface{ ValidCSRF(*http.Request) bool }
type ActorProvider interface {
	Actor(*http.Request) (string, bool)
}

// EnableHistoryDeletion exposes destructive history management behind the existing authorizer.
func (s *Server) EnableHistoryDeletion(store *storage.Manager, auth Authorizer, csrf CSRFValidator) {
	const prefix = "/api/v1/history/deletion-jobs"
	s.Handle("/api/v1/history/deletion-previews", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only POST is supported."})
			return
		}
		if auth == nil || !auth.Authorize(r) {
			WriteError(w, 401, Error{Code: "unauthorized", Message: "Authentication is required."})
			return
		}
		if csrf != nil && !csrf.ValidCSRF(r) {
			WriteError(w, 403, Error{Code: "csrf_invalid", Message: "A valid CSRF token is required."})
			return
		}
		var request storage.DeletionRequest
		if err := DecodeJSON(r, &request); err != nil {
			WriteError(w, 400, Error{Code: "invalid_request", Message: "The deletion scope is invalid."})
			return
		}
		preview, err := store.PreviewDeletion(r.Context(), request)
		if err != nil {
			WriteError(w, 400, Error{Code: "invalid_request", Message: "The deletion scope is invalid."})
			return
		}
		WriteJSON(w, 200, preview)
	}))
	s.Handle(prefix, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth == nil || !auth.Authorize(r) {
			WriteError(w, 401, Error{Code: "unauthorized", Message: "Authentication is required."})
			return
		}
		if r.Method != http.MethodPost {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only POST is supported."})
			return
		}
		if csrf != nil && !csrf.ValidCSRF(r) {
			WriteError(w, 403, Error{Code: "csrf_invalid", Message: "A valid CSRF token is required."})
			return
		}
		var body struct {
			Token        string `json:"token"`
			Confirmation string `json:"confirmation"`
		}
		if err := DecodeJSON(r, &body); err != nil {
			WriteError(w, 400, Error{Code: "invalid_request", Message: "A preview token and confirmation are required."})
			return
		}
		actor := ""
		if provider, ok := auth.(ActorProvider); ok {
			actor, _ = provider.Actor(r)
		}
		job, err := store.CreateDeletion(r.Context(), body.Token, body.Confirmation, actor)
		if err != nil {
			WriteError(w, 409, Error{Code: "deletion_not_available", Message: "The deletion cannot be started."})
			return
		}
		WriteJSON(w, 202, job)
	}))
	s.Handle(prefix+"/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth == nil || !auth.Authorize(r) {
			WriteError(w, 401, Error{Code: "unauthorized", Message: "Authentication is required."})
			return
		}
		tail := strings.TrimPrefix(r.URL.Path, prefix+"/")
		parts := strings.Split(tail, "/")
		if len(parts) == 0 || parts[0] == "" {
			WriteError(w, 404, Error{Code: "not_found", Message: "Deletion job not found."})
			return
		}
		id := parts[0]
		if len(parts) == 1 && r.Method == http.MethodGet {
			job, err := store.DeletionJob(r.Context(), id)
			if err != nil {
				WriteError(w, 404, Error{Code: "not_found", Message: "Deletion job not found."})
				return
			}
			WriteJSON(w, 200, job)
			return
		}
		if len(parts) == 2 && r.Method == http.MethodPost {
			if csrf != nil && !csrf.ValidCSRF(r) {
				WriteError(w, 403, Error{Code: "csrf_invalid", Message: "A valid CSRF token is required."})
				return
			}
			var err error
			if parts[1] == "cancel" {
				err = store.CancelDeletion(r.Context(), id)
			} else if parts[1] == "retry" {
				err = store.RetryDeletion(r.Context(), id)
			} else {
				WriteError(w, 404, Error{Code: "not_found", Message: "Deletion job not found."})
				return
			}
			if err != nil {
				WriteError(w, 409, Error{Code: "deletion_not_available", Message: "The deletion cannot be changed."})
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Unsupported deletion operation."})
	}))
}
