// SPDX-License-Identifier: AGPL-3.0-only

package storage_test

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"testing"

	"github.com/drilonrecica/binnacle/internal/alerts"
	"github.com/drilonrecica/binnacle/internal/storage"
	"github.com/drilonrecica/binnacle/migrations"
	_ "github.com/mattn/go-sqlite3"
)

func TestUpgradeSchema16PreservesExistingMonitoringData(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "binnacle.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, "PRAGMA foreign_keys=ON; CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	entries, err := fs.Glob(migrations.FS(), "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(entries)
	for _, entry := range entries {
		var version int
		if _, err = fmt.Sscanf(filepath.Base(entry), "%03d_", &version); err != nil {
			t.Fatal(err)
		}
		if version > 16 {
			break
		}
		body, readErr := migrations.FS().ReadFile(entry)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, err = db.ExecContext(ctx, string(body)); err != nil {
			t.Fatalf("apply migration %d: %v", version, err)
		}
		if _, err = db.ExecContext(ctx, "INSERT INTO schema_migrations(version, applied_at) VALUES(?, datetime('now'))", version); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.ExecContext(ctx, "INSERT INTO hosts(id,identity_hash,name,updated_at) VALUES('host-1','identity-1','existing','2026-01-01T00:00:00Z')"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO resources(id,host_id,stable_key,source_kind,name,category,status,first_seen_at,last_seen_at)
		VALUES('resource-1','host-1','stable-1','docker','existing','service','running',1,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO alert_rules(id,family,name,built_in,enabled,severity,scope_type,scope_id,trigger_seconds,recovery_seconds,window_seconds,cooldown_seconds,repeat_seconds,suppress_during_deployment,created_at,updated_at) VALUES('rule-1','test','Existing rule',0,1,'warning','resource','resource-1',0,0,0,300,7200,0,1,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO alerts(id,dedup_key,rule_id,family,severity,target_type,target_id,status,started_at,last_observed_at,message) VALUES('alert-1','test:resource-1','rule-1','test','warning','resource','resource-1','firing',1,1,'Existing alert')`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO health_checks(id,resource_id,name,url,method,interval_seconds,timeout_seconds,expected_status_min,expected_status_max,created_at,updated_at) VALUES('check-1','resource-1','Existing check','https://example.test','GET',30,5,200,299,1,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO settings(key,value_json,updated_at) VALUES('retention.preset','"balanced"',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO host_samples_10s(ts,host_id,cpu_busy_pct) VALUES(1,'host-1',12.5)`); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}

	manager := storage.New(dbPath, filepath.Join(dir, "runtime"))
	if err = manager.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if version, versionErr := manager.SchemaVersion(ctx); versionErr != nil || version != 18 {
		t.Fatalf("schema version=%d err=%v", version, versionErr)
	}
	var name string
	if err = manager.DB().QueryRowContext(ctx, "SELECT name FROM resources WHERE id='resource-1'").Scan(&name); err != nil || name != "existing" {
		t.Fatalf("existing data was not preserved: name=%q err=%v", name, err)
	}
	var resourceContext string
	if err = manager.DB().QueryRowContext(ctx, "SELECT context FROM resources WHERE id='resource-1'").Scan(&resourceContext); err != nil || resourceContext != "" {
		t.Fatalf("existing resource context=%q err=%v", resourceContext, err)
	}
	for table, id := range map[string]string{"alerts": "alert-1", "health_checks": "check-1"} {
		var count int
		if err = manager.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table+" WHERE id=?", id).Scan(&count); err != nil || count != 1 {
			t.Fatalf("%s was not preserved: count=%d err=%v", table, count, err)
		}
	}
	var setting string
	if err = manager.DB().QueryRowContext(ctx, `SELECT value_json FROM settings WHERE key='retention.preset'`).Scan(&setting); err != nil || setting != `"balanced"` {
		t.Fatalf("setting was not preserved: %q err=%v", setting, err)
	}
	var cpu float64
	if err = manager.DB().QueryRowContext(ctx, `SELECT cpu_busy_pct FROM host_samples_10s WHERE host_id='host-1' AND ts=1`).Scan(&cpu); err != nil || cpu != 12.5 {
		t.Fatalf("history was not preserved: %v err=%v", cpu, err)
	}

	repo := alerts.NewRepository(manager.DB())
	if err = repo.SeedDefaults(ctx); err != nil {
		t.Fatal(err)
	}
	if rules, rulesErr := repo.Rules(ctx); rulesErr != nil || len(rules) != len(alerts.DefaultRules())+1 {
		t.Fatalf("rules after upgrade=%d want=%d err=%v", len(rules), len(alerts.DefaultRules())+1, rulesErr)
	}
	if _, err = manager.DB().ExecContext(ctx, `INSERT INTO health_checks(id,resource_id,name,url,method,interval_seconds,timeout_seconds,expected_status_min,expected_status_max,created_at,updated_at)
		VALUES('bad','missing','bad','https://example.test','GET',30,5,200,299,1,1)`); err == nil {
		t.Fatal("foreign key constraint accepted a missing resource")
	}
	if _, err = manager.DB().ExecContext(ctx, `INSERT INTO health_checks(id,resource_id,name,url,method,interval_seconds,timeout_seconds,expected_status_min,expected_status_max,created_at,updated_at)
		VALUES('bad','resource-1','bad','https://example.test','POST',30,5,200,299,1,1)`); err == nil {
		t.Fatal("method constraint accepted POST")
	}
}
