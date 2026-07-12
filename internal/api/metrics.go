// SPDX-License-Identifier: AGPL-3.0-only
package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	authpkg "github.com/drilonrecica/binnacle/internal/auth"
	"github.com/drilonrecica/binnacle/internal/storage"
)

func (s *Server) EnableMetrics(store *storage.Manager, authz Authorizer, protection *authpkg.Protection) {
	s.Handle("/api/v1/metrics", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			WriteError(w, http.StatusMethodNotAllowed, Error{Code: "method_not_allowed", Message: "Only GET is supported."})
			return
		}
		if authz == nil || !authz.Authorize(r) {
			WriteError(w, http.StatusUnauthorized, Error{Code: "unauthorized", Message: "Authentication is required."})
			return
		}
		if ok, retry := protection.AllowMetrics(r); !ok {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", maxRetry(retry)))
			WriteError(w, http.StatusTooManyRequests, Error{Code: "rate_limited", Message: "Too many metric queries. Try again shortly.", Details: map[string]int{"retryAfterSeconds": maxRetry(retry)}})
			return
		}
		q := r.URL.Query()
		from, err := time.Parse(time.RFC3339, q.Get("from"))
		if err != nil {
			WriteError(w, 400, Error{Code: "invalid_time_range", Message: "A valid from timestamp is required."})
			return
		}
		to, err := time.Parse(time.RFC3339, q.Get("to"))
		if err != nil {
			WriteError(w, 400, Error{Code: "invalid_time_range", Message: "A valid to timestamp is required."})
			return
		}
		metrics := []storage.Metric{}
		for _, value := range strings.Split(q.Get("metrics"), ",") {
			if value = strings.TrimSpace(value); value != "" {
				metrics = append(metrics, storage.Metric(value))
			}
		}
		result, err := store.QueryMetrics(r.Context(), storage.MetricQuery{Scope: q.Get("scope"), ID: q.Get("id"), Metrics: metrics, From: from, To: to})
		if err != nil {
			WriteError(w, 400, Error{Code: "invalid_metrics_query", Message: "The metrics request is invalid."})
			return
		}
		WriteJSON(w, http.StatusOK, result)
	}))
}

func maxRetry(d time.Duration) int {
	if d < time.Second {
		return 1
	}
	return int(d.Round(time.Second).Seconds())
}
