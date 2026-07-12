// SPDX-License-Identifier: AGPL-3.0-only
package api

import (
	"github.com/drilonrecica/binnacle/internal/diagnostics"
	"net/http"
)

func (s *Server) EnableMonitorHealth(monitor *diagnostics.Monitor, authorizer Authorizer) {
	s.Handle("/api/v1/monitor-health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only GET is supported."})
			return
		}
		if authorizer == nil || !authorizer.Authorize(r) {
			WriteError(w, 401, Error{Code: "unauthorized", Message: "Authentication is required."})
			return
		}
		WriteJSON(w, 200, monitor.Snapshot())
	}))
}
