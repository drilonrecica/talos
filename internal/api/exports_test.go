// SPDX-License-Identifier: AGPL-3.0-only
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/drilonrecica/binnacle/internal/auth"
	"github.com/drilonrecica/binnacle/internal/metrics"
	"github.com/drilonrecica/binnacle/internal/notifications"
	"github.com/drilonrecica/binnacle/internal/storage"
)

func TestBoundedExportsHaveMetadataAndSafeAttachments(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	manager := storage.New(filepath.Join(dir, "db"), filepath.Join(dir, "run"))
	if err := manager.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	now := time.Now().UTC().Truncate(time.Second)
	_, err := manager.DB().ExecContext(ctx, "INSERT INTO hosts(id,identity_hash,name,updated_at) VALUES('host','hash','host',?)", now.Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.DB().ExecContext(ctx, "INSERT INTO host_samples_10s(ts,host_id,cpu_busy_pct) VALUES(?,?,?)", now.Add(-time.Minute).UnixMilli(), "host", 12.5)
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.DB().ExecContext(ctx, "INSERT INTO events(id,ts,type,severity,summary,source,created_at) VALUES('event',?,'deployment','info',?,'test',?)", now.UnixMilli(), "quoted \"summary\"", now.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	engine := metrics.NewEngine(10)
	engine.Publish(metrics.Snapshot{At: now, Resources: []metrics.ResourceSnapshot{{ID: "res_active", Name: "API", Status: metrics.StatusHealthy, Category: "application"}}})
	secrets, _ := auth.NewSecretStore(manager.DB(), "")
	incidents := notifications.NewRepository(manager.DB(), secrets)
	server := New()
	allow := DemoAuthorizer(true)
	server.EnableExports(manager, incidents, engine, allow, allow, allow, allow)
	from, to := url.QueryEscape(now.Add(-time.Hour).Format(time.RFC3339)), url.QueryEscape(now.Add(time.Minute).Format(time.RFC3339))
	cases := []struct{ path, filename, contentType string }{{"/api/v1/exports/metrics.csv?scope=host&metrics=cpu&from=" + from + "&to=" + to, "binnacle-metrics.csv", "text/csv"}, {"/api/v1/exports/events.json?from=" + from + "&to=" + to, "binnacle-events.json", "application/json"}, {"/api/v1/exports/incidents.json?from=" + from + "&to=" + to, "binnacle-incidents.json", "application/json"}, {"/api/v1/exports/resources.json", "binnacle-resources.json", "application/json"}}
	for _, test := range cases {
		request := httptest.NewRequest(http.MethodGet, "http://binnacle.test"+test.path, nil)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != 200 {
			t.Errorf("%s status=%d body=%s", test.path, response.Code, response.Body.String())
			continue
		}
		if !strings.Contains(response.Header().Get("Content-Disposition"), test.filename) || !strings.HasPrefix(response.Header().Get("Content-Type"), test.contentType) || response.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s headers=%v", test.path, response.Header())
		}
		if test.contentType == "application/json" {
			var envelope struct {
				SchemaVersion int             `json:"schemaVersion"`
				ExportedAt    time.Time       `json:"exportedAt"`
				Rows          json.RawMessage `json:"rows"`
			}
			if err = json.Unmarshal(response.Body.Bytes(), &envelope); err != nil || envelope.SchemaVersion != 1 || envelope.ExportedAt.Location() != time.UTC {
				t.Errorf("%s envelope=%+v err=%v", test.path, envelope, err)
			}
		} else if !strings.Contains(response.Body.String(), "schema_version,exported_at") || !strings.Contains(response.Body.String(), ",12.5,12.5,12.5,1") {
			t.Errorf("csv=%s", response.Body.String())
		}
	}
}

func TestExportRejectsRangesOverThirtyDays(t *testing.T) {
	server := New()
	server.EnableExports(nil, nil, nil, DemoAuthorizer(true), DemoAuthorizer(true), DemoAuthorizer(true), DemoAuthorizer(true))
	from := time.Now().Add(-31 * 24 * time.Hour).Format(time.RFC3339)
	to := time.Now().Format(time.RFC3339)
	request := httptest.NewRequest(http.MethodGet, "http://binnacle.test/api/v1/exports/events.json?from="+url.QueryEscape(from)+"&to="+url.QueryEscape(to), nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != 400 {
		t.Fatalf("status=%d", response.Code)
	}
}
