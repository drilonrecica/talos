// SPDX-License-Identifier: AGPL-3.0-only
package api

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/drilonrecica/binnacle/internal/metrics"
	"github.com/drilonrecica/binnacle/internal/notifications"
	"github.com/drilonrecica/binnacle/internal/storage"
)

const (
	maxExportRows       = 10000
	maxExportBytes      = 16 << 20
	exportSchemaVersion = 1
	maxExportRange      = 30 * 24 * time.Hour
)

type exportEnvelope struct {
	SchemaVersion int        `json:"schemaVersion"`
	ExportedAt    time.Time  `json:"exportedAt"`
	From          *time.Time `json:"from,omitempty"`
	To            *time.Time `json:"to,omitempty"`
	Rows          any        `json:"rows"`
}
type exportResource struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	Category    string     `json:"category,omitempty"`
	Context     string     `json:"context,omitempty"`
	Project     string     `json:"project,omitempty"`
	Environment string     `json:"environment,omitempty"`
	SourceKind  string     `json:"sourceKind,omitempty"`
	ArchivedAt  *time.Time `json:"archivedAt,omitempty"`
}

func (s *Server) EnableExports(store *storage.Manager, incidents *notifications.Repository, engine *metrics.Engine, metricsAuth, eventsAuth, incidentsAuth, resourcesAuth Authorizer, decorators ...SnapshotDecorator) {
	s.Handle("/api/v1/exports/metrics.csv", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only GET is supported."})
			return
		}
		if !requireAuth(w, r, metricsAuth) {
			return
		}
		from, to, ok := exportRange(w, r)
		if !ok {
			return
		}
		query := r.URL.Query()
		selected := []storage.Metric{}
		for _, raw := range strings.Split(query.Get("metrics"), ",") {
			if raw = strings.TrimSpace(raw); raw != "" {
				selected = append(selected, storage.Metric(raw))
			}
		}
		response, err := store.QueryMetrics(r.Context(), storage.MetricQuery{Scope: query.Get("scope"), ID: query.Get("id"), Metrics: selected, From: from, To: to})
		if err != nil {
			WriteError(w, 400, Error{Code: "invalid_export", Message: "The metrics export request is invalid."})
			return
		}
		var buffer bytes.Buffer
		writer := csv.NewWriter(&buffer)
		_ = writer.Write([]string{"schema_version", "exported_at", "scope", "id", "resolution", "metric", "unit", "timestamp", "min", "avg", "max", "count"})
		exported := time.Now().UTC().Format(time.RFC3339Nano)
		rows := 0
		for _, series := range response.Series {
			for _, point := range series.Points {
				rows++
				if rows > maxExportRows {
					WriteError(w, 413, Error{Code: "export_too_large", Message: "The export exceeds 10,000 rows."})
					return
				}
				_ = writer.Write([]string{strconv.Itoa(exportSchemaVersion), exported, response.Scope, response.ID, string(response.Resolution), string(series.Metric), series.Unit, point.At.UTC().Format(time.RFC3339Nano), floatString(point.Min), floatString(point.Avg), floatString(point.Max), strconv.Itoa(point.Count)})
			}
		}
		writer.Flush()
		if writer.Error() != nil {
			WriteError(w, 500, Error{Code: "export_failed", Message: "The metrics export could not be generated."})
			return
		}
		writeAttachment(w, "text/csv; charset=utf-8", "binnacle-metrics.csv", buffer.Bytes())
	}))
	s.Handle("/api/v1/exports/events.json", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only GET is supported."})
			return
		}
		if !requireAuth(w, r, eventsAuth) {
			return
		}
		from, to, ok := exportRange(w, r)
		if !ok {
			return
		}
		rows, err := store.ExportEvents(r.Context(), from, to, maxExportRows+1)
		if err != nil {
			WriteError(w, 500, Error{Code: "export_failed", Message: "Events could not be exported."})
			return
		}
		writeJSONExport(w, "binnacle-events.json", from, to, rows)
	}))
	s.Handle("/api/v1/exports/incidents.json", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only GET is supported."})
			return
		}
		if !requireAuth(w, r, incidentsAuth) {
			return
		}
		from, to, ok := exportRange(w, r)
		if !ok {
			return
		}
		rows, err := incidents.ExportIncidents(r.Context(), from, to, maxExportRows+1)
		if err != nil {
			WriteError(w, 500, Error{Code: "export_failed", Message: "Incidents could not be exported."})
			return
		}
		writeJSONExport(w, "binnacle-incidents.json", from, to, rows)
	}))
	s.Handle("/api/v1/exports/resources.json", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only GET is supported."})
			return
		}
		if !requireAuth(w, r, resourcesAuth) {
			return
		}
		snapshot := engine.Snapshot()
		for _, decorator := range decorators {
			snapshot = decorator.Decorate(r.Context(), snapshot)
		}
		rows := make([]exportResource, 0, len(snapshot.Resources))
		for _, value := range snapshot.Resources {
			rows = append(rows, exportResource{ID: string(value.ID), Name: value.Name, Status: string(value.Status), Category: value.Category, Context: value.Context, Project: value.Project, Environment: value.Environment, SourceKind: value.SourceKind})
		}
		archived, err := store.ArchivedResources(r.Context())
		if err != nil {
			WriteError(w, 500, Error{Code: "export_failed", Message: "Resources could not be exported."})
			return
		}
		for _, value := range archived {
			rows = append(rows, exportResource{ID: value.ID, Name: value.Name, Status: value.Status, Category: value.Category, Context: value.Context, Project: value.ProjectName, Environment: value.EnvironmentName, SourceKind: value.SourceKind, ArchivedAt: value.ArchivedAt})
		}
		if len(rows) > maxExportRows {
			WriteError(w, 413, Error{Code: "export_too_large", Message: "The export exceeds 10,000 rows."})
			return
		}
		writeJSONExport(w, "binnacle-resources.json", time.Time{}, time.Time{}, rows)
	}))
}
func exportRange(w http.ResponseWriter, r *http.Request) (time.Time, time.Time, bool) {
	from, err := time.Parse(time.RFC3339, r.URL.Query().Get("from"))
	if err != nil {
		WriteError(w, 400, Error{Code: "invalid_time_range", Message: "A valid from timestamp is required."})
		return time.Time{}, time.Time{}, false
	}
	to, err := time.Parse(time.RFC3339, r.URL.Query().Get("to"))
	if err != nil || !from.Before(to) || to.Sub(from) > maxExportRange {
		WriteError(w, 400, Error{Code: "invalid_time_range", Message: "Export ranges must be positive and no longer than 30 days."})
		return time.Time{}, time.Time{}, false
	}
	return from.UTC(), to.UTC(), true
}
func floatString(value *float64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatFloat(*value, 'g', -1, 64)
}
func writeJSONExport(w http.ResponseWriter, filename string, from, to time.Time, rows any) {
	length := 0
	switch values := rows.(type) {
	case []storage.HistoricalEvent:
		length = len(values)
	case []notifications.Incident:
		length = len(values)
	case []exportResource:
		length = len(values)
	}
	if length > maxExportRows {
		WriteError(w, 413, Error{Code: "export_too_large", Message: "The export exceeds 10,000 rows."})
		return
	}
	envelope := exportEnvelope{SchemaVersion: exportSchemaVersion, ExportedAt: time.Now().UTC(), Rows: rows}
	if !from.IsZero() {
		envelope.From = &from
		envelope.To = &to
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		WriteError(w, 500, Error{Code: "export_failed", Message: "The export could not be generated."})
		return
	}
	writeAttachment(w, "application/json; charset=utf-8", filename, append(body, '\n'))
}
func writeAttachment(w http.ResponseWriter, contentType, filename string, body []byte) {
	if len(body) > maxExportBytes {
		WriteError(w, 413, Error{Code: "export_too_large", Message: "The export exceeds 16 MiB."})
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
