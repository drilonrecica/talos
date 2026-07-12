// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"crypto/rand"
	"encoding/hex"
	"github.com/drilonrecica/binnacle/internal/alerts"
	"github.com/drilonrecica/binnacle/internal/auth"
	"github.com/drilonrecica/binnacle/internal/checks"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type checkInput struct {
	ResourceID        string `json:"resourceId"`
	Name              string `json:"name"`
	URL               string `json:"url"`
	Method            string `json:"method"`
	IntervalSeconds   int    `json:"intervalSeconds"`
	TimeoutSeconds    int    `json:"timeoutSeconds"`
	ExpectedStatusMin int    `json:"expectedStatusMin"`
	ExpectedStatusMax int    `json:"expectedStatusMax"`
	BodySubstring     string `json:"bodySubstring"`
	Required          bool   `json:"required"`
	Enabled           bool   `json:"enabled"`
}
type ruleInput struct {
	Family, Name, ScopeType, ScopeID                                               string
	Enabled                                                                        bool
	Severity                                                                       alerts.Severity
	Threshold, RecoveryThreshold                                                   *float64
	TriggerSeconds, RecoverySeconds, WindowSeconds, CooldownSeconds, RepeatSeconds int
	SuppressDuringDeployment                                                       bool
}

func (v ruleInput) rule(id string) alerts.Rule {
	return alerts.Rule{ID: id, Family: v.Family, Name: v.Name, Enabled: v.Enabled, Severity: v.Severity, ScopeType: v.ScopeType, ScopeID: v.ScopeID, Threshold: v.Threshold, RecoveryThreshold: v.RecoveryThreshold, TriggerDuration: time.Duration(v.TriggerSeconds) * time.Second, RecoveryDuration: time.Duration(v.RecoverySeconds) * time.Second, Window: time.Duration(v.WindowSeconds) * time.Second, Cooldown: time.Duration(v.CooldownSeconds) * time.Second, Repeat: time.Duration(v.RepeatSeconds) * time.Second, SuppressDuringDeployment: v.SuppressDuringDeployment}
}

func checkFromInput(id string, v checkInput) checks.Check {
	if v.Method == "" {
		v.Method = "GET"
	}
	if v.IntervalSeconds == 0 {
		v.IntervalSeconds = 30
	}
	if v.TimeoutSeconds == 0 {
		v.TimeoutSeconds = 5
	}
	if v.ExpectedStatusMin == 0 {
		v.ExpectedStatusMin = 200
	}
	if v.ExpectedStatusMax == 0 {
		v.ExpectedStatusMax = 399
	}
	return checks.Check{ID: id, ResourceID: v.ResourceID, Name: v.Name, URL: v.URL, Method: v.Method, Interval: time.Duration(v.IntervalSeconds) * time.Second, Timeout: time.Duration(v.TimeoutSeconds) * time.Second, ExpectedStatusMin: v.ExpectedStatusMin, ExpectedStatusMax: v.ExpectedStatusMax, BodySubstring: v.BodySubstring, Required: v.Required, Enabled: v.Enabled}
}
func apiID(prefix string) string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}
func authorizeMutation(w http.ResponseWriter, r *http.Request, a Authorizer, s *auth.Sessions) bool {
	if a == nil || !a.Authorize(r) {
		WriteError(w, 401, Error{Code: "unauthorized", Message: "Authentication is required."})
		return false
	}
	if !s.ValidCSRF(r) {
		WriteError(w, 403, Error{Code: "csrf_invalid", Message: "A valid CSRF token is required."})
		return false
	}
	return true
}
func pagination(r *http.Request) (int, int) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit < 1 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func (s *Server) EnableChecks(repo *checks.Repository, scheduler *checks.Scheduler, a Authorizer, sessions *auth.Sessions, protections ...*auth.Protection) {
	s.Handle("/api/v1/checks", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if a == nil || !a.Authorize(r) {
				WriteError(w, 401, Error{Code: "unauthorized", Message: "Authentication is required."})
				return
			}
			if len(protections) > 0 {
				if ok, retry := protections[0].AllowResources(r); !ok {
					w.Header().Set("Retry-After", strconv.Itoa(maxRetry(retry)))
					WriteError(w, 429, Error{Code: "rate_limited", Message: "Too many check requests."})
					return
				}
			}
			limit, offset := pagination(r)
			v, err := repo.List(r.Context(), limit, offset)
			if err != nil {
				WriteError(w, 500, Error{Code: "storage_error", Message: "Checks are unavailable."})
				return
			}
			WriteJSON(w, 200, v)
		case http.MethodPost:
			if !authorizeMutation(w, r, a, sessions) {
				return
			}
			var body checkInput
			if DecodeJSON(r, &body) != nil {
				WriteError(w, 400, Error{Code: "invalid_request", Message: "Invalid check definition."})
				return
			}
			c := checkFromInput(apiID("check_"), body)
			if err := c.Validate(); err != nil {
				WriteError(w, 400, Error{Code: "invalid_request", Message: err.Error()})
				return
			}
			if err := repo.Create(r.Context(), c); err != nil {
				WriteError(w, 400, Error{Code: "invalid_request", Message: "The check could not be created."})
				return
			}
			WriteJSON(w, 201, c)
		default:
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Method not allowed."})
		}
	}))
	s.Handle("/api/v1/checks/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/checks/")
		run := strings.HasSuffix(path, "/run")
		id := strings.TrimSuffix(path, "/run")
		if id == "" || strings.Contains(id, "/") {
			WriteError(w, 404, Error{Code: "not_found", Message: "Check not found."})
			return
		}
		if run {
			if r.Method != http.MethodPost || !authorizeMutation(w, r, a, sessions) {
				if r.Method != http.MethodPost {
					WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only POST is supported."})
				}
				return
			}
			result, err := scheduler.RunNow(r.Context(), id)
			if err != nil {
				WriteError(w, 400, Error{Code: "check_failed", Message: "The check could not be run."})
				return
			}
			WriteJSON(w, 200, result)
			return
		}
		switch r.Method {
		case http.MethodGet:
			if a == nil || !a.Authorize(r) {
				WriteError(w, 401, Error{Code: "unauthorized", Message: "Authentication is required."})
				return
			}
			v, err := repo.Get(r.Context(), id)
			if err != nil {
				WriteError(w, 404, Error{Code: "not_found", Message: "Check not found."})
				return
			}
			WriteJSON(w, 200, v)
		case http.MethodPatch:
			if !authorizeMutation(w, r, a, sessions) {
				return
			}
			existing, err := repo.Get(r.Context(), id)
			if err != nil {
				WriteError(w, 404, Error{Code: "not_found", Message: "Check not found."})
				return
			}
			var body checkInput
			if DecodeJSON(r, &body) != nil {
				WriteError(w, 400, Error{Code: "invalid_request", Message: "Invalid check definition."})
				return
			}
			updated := checkFromInput(id, body)
			updated.CreatedAt = existing.CreatedAt
			if err = repo.Update(r.Context(), updated); err != nil {
				WriteError(w, 400, Error{Code: "invalid_request", Message: "The check could not be updated."})
				return
			}
			WriteJSON(w, 200, updated)
		case http.MethodDelete:
			if !authorizeMutation(w, r, a, sessions) {
				return
			}
			if err := repo.Delete(r.Context(), id); err != nil {
				WriteError(w, 500, Error{Code: "storage_error", Message: "The check could not be deleted."})
				return
			}
			w.WriteHeader(204)
		default:
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Method not allowed."})
		}
	}))
}

func (s *Server) EnableAlerts(repo *alerts.Repository, a Authorizer, sessions *auth.Sessions, protections ...*auth.Protection) {
	s.Handle("/api/v1/alerts", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only GET is supported."})
			return
		}
		if a == nil || !a.Authorize(r) {
			WriteError(w, 401, Error{Code: "unauthorized", Message: "Authentication is required."})
			return
		}
		limit, offset := pagination(r)
		q := r.URL.Query()
		v, err := repo.Alerts(r.Context(), q.Get("status"), q.Get("severity"), q.Get("resource"), q.Get("family"), limit, offset)
		if err != nil {
			WriteError(w, 500, Error{Code: "storage_error", Message: "Alerts are unavailable."})
			return
		}
		WriteJSON(w, 200, v)
	}))
	s.Handle("/api/v1/alert-rules", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if !authorizeMutation(w, r, a, sessions) {
				return
			}
			var body ruleInput
			if DecodeJSON(r, &body) != nil {
				WriteError(w, 400, Error{Code: "invalid_request", Message: "Invalid rule definition."})
				return
			}
			rule := body.rule(apiID("rule_"))
			if err := repo.CreateRule(r.Context(), rule); err != nil {
				WriteError(w, 400, Error{Code: "invalid_request", Message: "Rule could not be created."})
				return
			}
			WriteJSON(w, 201, rule)
			return
		}
		if r.Method != http.MethodGet {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Method not allowed."})
			return
		}
		if a == nil || !a.Authorize(r) {
			WriteError(w, 401, Error{Code: "unauthorized", Message: "Authentication is required."})
			return
		}
		v, err := repo.Rules(r.Context())
		if err != nil {
			WriteError(w, 500, Error{Code: "storage_error", Message: "Rules are unavailable."})
			return
		}
		WriteJSON(w, 200, v)
	}))
	s.Handle("/api/v1/alert-rules/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only PATCH is supported."})
			return
		}
		if !authorizeMutation(w, r, a, sessions) {
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/alert-rules/")
		var body struct {
			Enabled           *bool           `json:"enabled"`
			Severity          alerts.Severity `json:"severity"`
			Threshold         *float64        `json:"threshold"`
			RecoveryThreshold *float64        `json:"recoveryThreshold"`
			TriggerSeconds    *int            `json:"triggerSeconds"`
			RecoverySeconds   *int            `json:"recoverySeconds"`
		}
		if DecodeJSON(r, &body) != nil {
			WriteError(w, 400, Error{Code: "invalid_request", Message: "Invalid rule update."})
			return
		}
		rules, err := repo.Rules(r.Context())
		if err != nil {
			WriteError(w, 500, Error{Code: "storage_error", Message: "Rule unavailable."})
			return
		}
		var found *alerts.Rule
		for i := range rules {
			if rules[i].ID == id {
				found = &rules[i]
				break
			}
		}
		if found == nil {
			WriteError(w, 404, Error{Code: "not_found", Message: "Rule not found."})
			return
		}
		if body.Enabled != nil {
			found.Enabled = *body.Enabled
		}
		if body.Severity != "" {
			found.Severity = body.Severity
		}
		if body.Threshold != nil {
			found.Threshold = body.Threshold
		}
		if body.RecoveryThreshold != nil {
			found.RecoveryThreshold = body.RecoveryThreshold
		}
		if body.TriggerSeconds != nil {
			found.TriggerDuration = time.Duration(*body.TriggerSeconds) * time.Second
		}
		if body.RecoverySeconds != nil {
			found.RecoveryDuration = time.Duration(*body.RecoverySeconds) * time.Second
		}
		if err = repo.UpdateRule(r.Context(), *found); err != nil {
			WriteError(w, 400, Error{Code: "invalid_request", Message: "Rule could not be updated."})
			return
		}
		WriteJSON(w, 200, found)
	}))
	s.Handle("/api/v1/silences", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if a == nil || !a.Authorize(r) {
				WriteError(w, 401, Error{Code: "unauthorized", Message: "Authentication is required."})
				return
			}
			v, err := repo.Silences(r.Context(), r.URL.Query().Get("active") == "true")
			if err != nil {
				WriteError(w, 500, Error{Code: "storage_error", Message: "Silences are unavailable."})
				return
			}
			WriteJSON(w, 200, v)
		case http.MethodPost:
			if !authorizeMutation(w, r, a, sessions) {
				return
			}
			var body struct {
				alerts.Silence
				Preset    string    `json:"preset"`
				CustomEnd time.Time `json:"customEnd"`
			}
			if DecodeJSON(r, &body) != nil {
				WriteError(w, 400, Error{Code: "invalid_request", Message: "Invalid silence."})
				return
			}
			v := body.Silence
			v.CreatedBy = "admin"
			if v.StartsAt.IsZero() {
				v.StartsAt = time.Now().UTC()
			}
			if body.Preset != "" {
				end, err := alerts.SilencePresetEnd(v.StartsAt, body.Preset, body.CustomEnd)
				if err != nil {
					WriteError(w, 400, Error{Code: "invalid_request", Message: "Invalid silence end."})
					return
				}
				v.EndsAt = end
			}
			if err := repo.CreateSilence(r.Context(), &v); err != nil {
				WriteError(w, 400, Error{Code: "invalid_request", Message: "Silence could not be created."})
				return
			}
			WriteJSON(w, 201, v)
		default:
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Method not allowed."})
		}
	}))
	s.Handle("/api/v1/silences/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only DELETE is supported."})
			return
		}
		if !authorizeMutation(w, r, a, sessions) {
			return
		}
		err := repo.DeleteSilence(r.Context(), strings.TrimPrefix(r.URL.Path, "/api/v1/silences/"))
		if err != nil {
			WriteError(w, 404, Error{Code: "not_found", Message: "Silence not found."})
			return
		}
		w.WriteHeader(204)
	}))
}
