// SPDX-License-Identifier: AGPL-3.0-only
package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/drilonrecica/talos/internal/auth"
	"github.com/drilonrecica/talos/internal/diagnostics"
)

func (s *Server) EnableDiagnostics(service *diagnostics.BundleService, authorizer Authorizer, protection *auth.Protection) {
	const prefix = "/api/v1/diagnostics/previews/"
	s.Handle("/api/v1/diagnostics/previews", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only POST is supported."})
			return
		}
		if authorizer == nil || !authorizer.Authorize(r) {
			WriteError(w, 401, Error{Code: "unauthorized", Message: "Authentication is required."})
			return
		}
		actor := "admin"
		if provider, ok := authorizer.(ActorProvider); ok {
			if value, found := provider.Actor(r); found {
				actor = value
			}
		}
		if protection != nil {
			if ok, retry := protection.AllowDiagnostics(r, actor); !ok {
				seconds := maxRetry(retry)
				w.Header().Set("Retry-After", fmt.Sprint(seconds))
				WriteError(w, 429, Error{Code: "rate_limited", Message: "Diagnostics generation is temporarily limited.", Details: map[string]int{"retryAfterSeconds": seconds}})
				return
			}
		}
		preview, err := service.Generate(r.Context())
		if err != nil {
			WriteError(w, 500, Error{Code: "diagnostics_error", Message: "Diagnostics could not be generated."})
			return
		}
		WriteJSON(w, 201, preview)
	}))
	s.Handle(prefix, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only GET is supported."})
			return
		}
		if authorizer == nil || !authorizer.Authorize(r) {
			WriteError(w, 401, Error{Code: "unauthorized", Message: "Authentication is required."})
			return
		}
		tail := strings.TrimPrefix(r.URL.Path, prefix)
		if !strings.HasSuffix(tail, "/download") {
			WriteError(w, 404, Error{Code: "not_found", Message: "Diagnostics preview not found."})
			return
		}
		id := strings.TrimSuffix(tail, "/download")
		archive, _, err := service.Download(id)
		if err != nil {
			WriteError(w, 404, Error{Code: "not_found", Message: "Diagnostics preview expired or was not found."})
			return
		}
		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", `attachment; filename="talos-diagnostics.tar.gz"`)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(archive)
	}))
}
