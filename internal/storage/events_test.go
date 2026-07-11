// SPDX-License-Identifier: AGPL-3.0-only
package storage

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEventsForResourceUseStableJSONFields(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m := New(filepath.Join(dir, "talos.db"), filepath.Join(dir, "run"))
	if err := m.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	now := time.Now().UTC()
	for _, item := range []struct{ id, resource string }{{"one", "res_one"}, {"two", "res_two"}} {
		if _, err := m.db.ExecContext(ctx, "INSERT INTO events(id,ts,resource_id,type,severity,summary,source,created_at) VALUES(?,?,?,?,?,?,?,?)", item.id, now.UnixMilli(), item.resource, "deployment", "info", "Deployed", "docker", now.UnixMilli()); err != nil {
			t.Fatal(err)
		}
	}
	events, err := m.EventsFor(ctx, now.Add(-time.Minute), now.Add(time.Minute), 100, "res_one")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ID != "one" {
		t.Fatalf("events=%+v", events)
	}
	encoded, _ := json.Marshal(events[0])
	text := string(encoded)
	for _, field := range []string{`"type":"deployment"`, `"severity":"info"`, `"summary":"Deployed"`, `"source":"docker"`} {
		if !strings.Contains(text, field) {
			t.Fatalf("json=%s missing %s", text, field)
		}
	}
}
