// SPDX-License-Identifier: AGPL-3.0-only
package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/drilonrecica/binnacle/internal/diagnostics"
)

func (s *Server) EnableProcesses(scanner *diagnostics.ProcessScanner, sessions Authorizer) {
	s.Handle("/api/v1/processes", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only GET is supported."})
			return
		}
		if sessions == nil || !sessions.Authorize(r) {
			WriteError(w, 401, Error{Code: "unauthorized", Message: "A browser session is required."})
			return
		}
		limit := 0
		if raw := r.URL.Query().Get("limit"); raw != "" {
			var err error
			limit, err = strconv.Atoi(raw)
			if err != nil {
				WriteError(w, 400, Error{Code: "invalid_request", Message: "limit must be an integer."})
				return
			}
		}
		values, err := scanner.Scan(r.Context(), limit)
		if errors.Is(err, diagnostics.ErrProcessScanBusy) {
			WriteError(w, 429, Error{Code: "scan_busy", Message: "A process scan is already running."})
			return
		}
		if err != nil {
			WriteError(w, 503, Error{Code: "processes_unavailable", Message: "Host processes are unavailable."})
			return
		}
		WriteJSON(w, 200, map[string]any{"processes": values, "sampled": true})
	}))
}
