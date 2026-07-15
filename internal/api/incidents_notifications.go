// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/drilonrecica/binnacle/internal/auth"
	"github.com/drilonrecica/binnacle/internal/notifications"
)

type channelInput struct {
	Name            string    `json:"name"`
	Kind            string    `json:"kind"`
	Enabled         *bool     `json:"enabled"`
	MinimumSeverity string    `json:"minimumSeverity"`
	NotifyResolved  *bool     `json:"notifyResolved"`
	TLSMode         string    `json:"tlsMode"`
	URL             *string   `json:"url"`
	BearerToken     *string   `json:"bearerToken"`
	SigningSecret   *string   `json:"signingSecret"`
	Host            *string   `json:"host"`
	Username        *string   `json:"username"`
	Password        *string   `json:"password"`
	Sender          *string   `json:"sender"`
	Recipients      *[]string `json:"recipients"`
}

func boolValue(v *bool, def bool) bool {
	if v == nil {
		return def
	}
	return *v
}
func (v channelInput) channel() notifications.Channel {
	return notifications.Channel{Name: v.Name, Kind: v.Kind, Enabled: boolValue(v.Enabled, true), MinimumSeverity: v.MinimumSeverity, NotifyResolved: boolValue(v.NotifyResolved, true), Config: map[string]any{"tlsMode": v.TLSMode}}
}
func str(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
func (v channelInput) secrets() notifications.ChannelSecrets {
	return notifications.ChannelSecrets{URL: str(v.URL), BearerToken: str(v.BearerToken), SigningSecret: str(v.SigningSecret), Host: str(v.Host), Username: str(v.Username), Password: str(v.Password), Sender: str(v.Sender), Recipients: func() []string {
		if v.Recipients == nil {
			return nil
		}
		return *v.Recipients
	}()}
}
func (v channelInput) patch() notifications.SecretPatch {
	return notifications.SecretPatch{URL: v.URL, BearerToken: v.BearerToken, SigningSecret: v.SigningSecret, Host: v.Host, Username: v.Username, Password: v.Password, Sender: v.Sender, Recipients: v.Recipients}
}

func allowNotificationAPI(w http.ResponseWriter, r *http.Request, protections []*auth.Protection) bool {
	if len(protections) == 0 || protections[0] == nil {
		return true
	}
	if ok, retry := protections[0].AllowResources(r); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(maxRetry(retry)))
		WriteError(w, 429, Error{Code: "rate_limited", Message: "Too many incident or notification requests."})
		return false
	}
	return true
}

func (s *Server) EnableIncidentsNotifications(repo *notifications.Repository, worker *notifications.Worker, a Authorizer, sessions *auth.Sessions, protections ...*auth.Protection) {
	s.Handle("/api/v1/incidents", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requireAuth(w, r, a) {
			return
		}
		if !allowNotificationAPI(w, r, protections) {
			return
		}
		if r.Method != http.MethodGet {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only GET is supported."})
			return
		}
		limit, offset := pagination(r)
		v, err := repo.Incidents(r.Context(), r.URL.Query().Get("status"), r.URL.Query().Get("severity"), limit, offset)
		if err != nil {
			WriteError(w, 500, Error{Code: "storage_error", Message: "Incidents are unavailable."})
			return
		}
		WriteJSON(w, 200, v)
	}))
	s.Handle("/api/v1/incidents/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requireAuth(w, r, a) {
			return
		}
		if !allowNotificationAPI(w, r, protections) {
			return
		}
		if r.Method != http.MethodGet {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only GET is supported."})
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/incidents/")
		if id == "" || strings.Contains(id, "/") {
			WriteError(w, 404, Error{Code: "not_found", Message: "Incident not found."})
			return
		}
		v, err := repo.Incident(r.Context(), id)
		if err != nil {
			WriteError(w, 404, Error{Code: "not_found", Message: "Incident not found."})
			return
		}
		WriteJSON(w, 200, v)
	}))
	s.Handle("/api/v1/notification-channels", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requireAuth(w, r, a) {
			return
		}
		if !allowNotificationAPI(w, r, protections) {
			return
		}
		switch r.Method {
		case http.MethodGet:
			v, err := repo.Channels(r.Context())
			if err != nil {
				WriteError(w, 500, Error{Code: "storage_error", Message: "Notification channels are unavailable."})
				return
			}
			WriteJSON(w, 200, v)
		case http.MethodPost:
			if !authorizeMutation(w, r, a, sessions) {
				return
			}
			var body channelInput
			if DecodeJSON(r, &body) != nil {
				WriteError(w, 400, Error{Code: "invalid_request", Message: "Invalid notification channel."})
				return
			}
			c := body.channel()
			secret := body.secrets()
			if err := worker.ValidateChannel(r.Context(), c, secret); err != nil {
				WriteError(w, 400, Error{Code: "invalid_target", Message: err.Error()})
				return
			}
			created, err := repo.CreateChannel(r.Context(), c, secret)
			if err != nil {
				code := "invalid_request"
				message := "The notification channel could not be created."
				if errors.Is(err, auth.ErrMasterKeyMissing) {
					code = "master_key_missing"
					message = "BINNACLE_MASTER_KEY or BINNACLE_MASTER_KEY_FILE is required to configure notification channels."
				}
				WriteError(w, 400, Error{Code: code, Message: message})
				return
			}
			WriteJSON(w, 201, created)
		default:
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Method not allowed."})
		}
	}))
	s.Handle("/api/v1/notification-channels/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requireAuth(w, r, a) {
			return
		}
		if !allowNotificationAPI(w, r, protections) {
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/notification-channels/")
		test := strings.HasSuffix(path, "/test")
		id := strings.TrimSuffix(path, "/test")
		if id == "" || strings.Contains(id, "/") {
			WriteError(w, 404, Error{Code: "not_found", Message: "Notification channel not found."})
			return
		}
		if test {
			if r.Method != http.MethodPost {
				WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only POST is supported."})
				return
			}
			if !authorizeMutation(w, r, a, sessions) {
				return
			}
			deliveryID, err := repo.Test(r.Context(), id)
			if err != nil {
				WriteError(w, 404, Error{Code: "not_found", Message: "Notification channel not found."})
				return
			}
			WriteJSON(w, 202, map[string]string{"deliveryId": deliveryID})
			return
		}
		switch r.Method {
		case http.MethodGet:
			v, err := repo.Channel(r.Context(), id)
			if err != nil {
				WriteError(w, 404, Error{Code: "not_found", Message: "Notification channel not found."})
				return
			}
			WriteJSON(w, 200, v)
		case http.MethodPatch:
			if !authorizeMutation(w, r, a, sessions) {
				return
			}
			existing, err := repo.Channel(r.Context(), id)
			if err != nil {
				WriteError(w, 404, Error{Code: "not_found", Message: "Notification channel not found."})
				return
			}
			var body channelInput
			if DecodeJSON(r, &body) != nil {
				WriteError(w, 400, Error{Code: "invalid_request", Message: "Invalid notification channel."})
				return
			}
			c := body.channel()
			c.Kind = existing.Kind
			if c.Name == "" {
				c.Name = existing.Name
			}
			if body.Enabled == nil {
				c.Enabled = existing.Enabled
			}
			if body.NotifyResolved == nil {
				c.NotifyResolved = existing.NotifyResolved
			}
			if body.MinimumSeverity == "" {
				c.MinimumSeverity = existing.MinimumSeverity
			}
			if body.TLSMode == "" {
				c.Config = existing.Config
			}
			if body.URL != nil || body.Host != nil {
				candidate, candidateErr := repo.PatchedSecrets(r.Context(), id, body.patch())
				if candidateErr != nil || worker.ValidateChannel(r.Context(), c, candidate) != nil {
					WriteError(w, 400, Error{Code: "invalid_target", Message: "The notification target is invalid or blocked."})
					return
				}
			}
			updated, err := repo.PatchChannel(r.Context(), id, c, body.patch())
			if err != nil {
				WriteError(w, 400, Error{Code: "invalid_request", Message: "The notification channel could not be updated."})
				return
			}
			WriteJSON(w, 200, updated)
		case http.MethodDelete:
			if !authorizeMutation(w, r, a, sessions) {
				return
			}
			if err := repo.DeleteChannel(r.Context(), id); err != nil {
				WriteError(w, 404, Error{Code: "not_found", Message: "Notification channel not found."})
				return
			}
			w.WriteHeader(204)
		default:
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Method not allowed."})
		}
	}))
	s.Handle("/api/v1/notification-deliveries", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requireAuth(w, r, a) {
			return
		}
		if !allowNotificationAPI(w, r, protections) {
			return
		}
		if r.Method != http.MethodGet {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only GET is supported."})
			return
		}
		limit, offset := pagination(r)
		v, err := repo.Deliveries(r.Context(), r.URL.Query().Get("incidentId"), limit, offset)
		if err != nil {
			WriteError(w, 500, Error{Code: "storage_error", Message: "Delivery history is unavailable."})
			return
		}
		WriteJSON(w, 200, v)
	}))
	s.Handle("/api/v1/notification-deliveries/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requireAuth(w, r, a) {
			return
		}
		if !allowNotificationAPI(w, r, protections) {
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/notification-deliveries/")
		if !strings.HasSuffix(path, "/retry") || r.Method != http.MethodPost {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only retry POST is supported."})
			return
		}
		if !authorizeMutation(w, r, a, sessions) {
			return
		}
		id := strings.TrimSuffix(path, "/retry")
		if err := repo.Retry(r.Context(), id); err != nil {
			status := 400
			if errors.Is(err, sql.ErrNoRows) {
				status = 404
			}
			WriteError(w, status, Error{Code: "not_retryable", Message: "Delivery is not available for retry."})
			return
		}
		WriteJSON(w, 202, map[string]string{"deliveryId": id})
	}))
}
