// SPDX-License-Identifier: AGPL-3.0-only
package api

import (
	"github.com/drilonrecica/talos/internal/storage"
	"net/http"
	"time"
)

func (s *Server) EnableEvents(store *storage.Manager, auth Authorizer) {
	s.Handle("/api/v1/events", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only GET is supported."})
			return
		}
		if auth == nil || !auth.Authorize(r) {
			WriteError(w, 401, Error{Code: "unauthorized", Message: "Authentication is required."})
			return
		}
		to := time.Now().UTC()
		from := to.Add(-24 * time.Hour)
		if raw := r.URL.Query().Get("from"); raw != "" {
			var e error
			from, e = time.Parse(time.RFC3339, raw)
			if e != nil {
				WriteError(w, 400, Error{Code: "invalid_time_range", Message: "Invalid from timestamp."})
				return
			}
		}
		if raw := r.URL.Query().Get("to"); raw != "" {
			var e error
			to, e = time.Parse(time.RFC3339, raw)
			if e != nil || !from.Before(to) {
				WriteError(w, 400, Error{Code: "invalid_time_range", Message: "Invalid to timestamp."})
				return
			}
		}
		v, e := store.EventsFor(r.Context(), from, to, 100, r.URL.Query().Get("resource_id"))
		if e != nil {
			WriteError(w, 500, Error{Code: "storage_error", Message: "Event history is unavailable."})
			return
		}
		WriteJSON(w, 200, v)
	}))
}
