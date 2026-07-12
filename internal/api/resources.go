// SPDX-License-Identifier: AGPL-3.0-only
package api

import (
	"fmt"
	"net/http"
	"strings"

	authpkg "github.com/drilonrecica/binnacle/internal/auth"
	"github.com/drilonrecica/binnacle/internal/metrics"
	"github.com/drilonrecica/binnacle/internal/storage"
)

func (s *Server) EnableResources(engine *metrics.Engine, auth Authorizer, store *storage.Manager, protection *authpkg.Protection) {
	s.Handle("/api/v1/resources", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth == nil || !auth.Authorize(r) {
			WriteError(w, 401, Error{Code: "unauthorized", Message: "Authentication is required."})
			return
		}
		if ok, retry := protection.AllowResources(r); !ok {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", maxRetry(retry)))
			WriteError(w, 429, Error{Code: "rate_limited", Message: "Too many resource requests. Try again shortly.", Details: map[string]int{"retryAfterSeconds": maxRetry(retry)}})
			return
		}
		snap := engine.Snapshot()
		if r.URL.Path == "/api/v1/resources" && r.URL.Query().Get("state") == "archived" {
			values, err := store.ArchivedResources(r.Context())
			if err != nil {
				WriteError(w, 500, Error{Code: "storage_error", Message: "Archived resources are unavailable."})
				return
			}
			WriteJSON(w, 200, values)
			return
		}
		if r.URL.Path != "/api/v1/resources" {
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/resources/")
			for _, v := range snap.Resources {
				if string(v.ID) == id {
					WriteJSON(w, 200, v)
					return
				}
			}
			if value, err := store.Resource(r.Context(), id); err == nil {
				WriteJSON(w, 200, value)
				return
			}
			WriteError(w, 404, Error{Code: "not_found", Message: "Resource not found."})
			return
		}
		WriteJSON(w, 200, snap.Resources)
	}))
}
