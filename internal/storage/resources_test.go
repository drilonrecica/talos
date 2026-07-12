// SPDX-License-Identifier: AGPL-3.0-only
package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestArchivedResourcesRemainDiscoverableAndCanReactivate(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	manager := New(filepath.Join(dir, "binnacle.db"), filepath.Join(dir, "run"))
	if err := manager.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if err := manager.UpsertHost(ctx, "host", HostIdentity("", "test"), "Server"); err != nil {
		t.Fatal(err)
	}
	resource := Resource{ID: "res_archived", HostID: "host", StableKey: "compose:project:web", SourceKind: "compose", Name: "Web", ProjectName: "Project", Category: "service", Status: "healthy"}
	if err := manager.UpsertResource(ctx, resource); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.db.ExecContext(ctx, "UPDATE resources SET last_seen_at=? WHERE id=?", time.Now().Add(-10*time.Minute).UnixMilli(), resource.ID); err != nil {
		t.Fatal(err)
	}
	if err := manager.ArchiveMissingResources(ctx, nil, time.Now().Add(-5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	archived, err := manager.ArchivedResources(ctx)
	if err != nil || len(archived) != 1 || archived[0].ProjectName != "Project" || archived[0].ArchivedAt == nil {
		t.Fatalf("archived=%+v err=%v", archived, err)
	}
	if err = manager.UpsertResource(ctx, resource); err != nil {
		t.Fatal(err)
	}
	value, err := manager.Resource(ctx, resource.ID)
	if err != nil || value.Status != "healthy" || value.ArchivedAt != nil {
		t.Fatalf("resource=%+v err=%v", value, err)
	}
}
