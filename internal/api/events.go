// SPDX-License-Identifier: AGPL-3.0-only
package api

import (
	"fmt"
	"net/http"
	"time"

	authpkg "github.com/drilonrecica/binnacle/internal/auth"
	"github.com/drilonrecica/binnacle/internal/storage"
)

const maxEventRange = 7 * 24 * time.Hour

func (s *Server) EnableEvents(store *storage.Manager, auth Authorizer, protection *authpkg.Protection) {
	s.Handle("/api/v1/events", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only GET is supported."})
			return
		}
		if auth == nil || !auth.Authorize(r) {
			WriteError(w, 401, Error{Code: "unauthorized", Message: "Authentication is required."})
			return
		}
		if ok, retry := protection.AllowEvents(r); !ok {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", maxRetry(retry)))
			WriteError(w, 429, Error{Code: "rate_limited", Message: "Too many event queries. Try again shortly.", Details: map[string]int{"retryAfterSeconds": maxRetry(retry)}})
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
		if to.Sub(from) > maxEventRange {
			WriteError(w, 400, Error{Code: "invalid_time_range", Message: "Event queries are limited to 7 days."})
			return
		}
		v, e := store.EventsFor(r.Context(), from, to, 100, r.URL.Query().Get("resource_id"))
		if e != nil {
			WriteError(w, 500, Error{Code: "storage_error", Message: "Event history is unavailable."})
			return
		}
		WriteJSON(w, 200, v)
	}))
}
