// SPDX-License-Identifier: AGPL-3.0-only
package api

import (
	"github.com/drilonrecica/binnacle/internal/metrics"
	"net/http"
)

func (s *Server) EnableCurrent(engine *metrics.Engine, auth Authorizer) {
	guard := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only GET is supported."})
				return
			}
			if auth == nil || !auth.Authorize(r) {
				WriteError(w, 401, Error{Code: "unauthorized", Message: "Authentication is required."})
				return
			}
			next(w, r)
		}
	}
	s.Handle("/api/v1/server", guard(func(w http.ResponseWriter, r *http.Request) {
		snap := engine.Snapshot()
		WriteJSON(w, 200, struct {
			At   string                  `json:"at"`
			Boot metrics.BootIdentity    `json:"bootIdentity"`
			Host metrics.HostObservation `json:"host"`
		}{snap.At.UTC().Format("2006-01-02T15:04:05Z07:00"), snap.BootIdentity, snap.Host})
	}))
	s.Handle("/api/v1/collector-health", guard(func(w http.ResponseWriter, r *http.Request) { WriteJSON(w, 200, engine.Snapshot().Collectors) }))
}
