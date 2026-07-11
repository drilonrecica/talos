// SPDX-License-Identifier: AGPL-3.0-only
package storage

import (
	"context"
	"fmt"
	"time"
)

type Resource struct {
	ID, HostID, StableKey, SourceKind, Name, Category, Status string
	ArchivedAt                                                *time.Time
}

func (m *Manager) UpsertResource(ctx context.Context, r Resource) error {
	if m.db == nil {
		return fmt.Errorf("storage is not open")
	}
	now := time.Now().UnixMilli()
	_, e := m.db.ExecContext(ctx, "INSERT INTO resources(id,host_id,stable_key,source_kind,name,category,status,first_seen_at,last_seen_at) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(host_id,stable_key) DO UPDATE SET name=excluded.name,category=excluded.category,status=excluded.status,last_seen_at=excluded.last_seen_at", r.ID, r.HostID, r.StableKey, r.SourceKind, r.Name, r.Category, r.Status, now, now)
	return e
}
func (m *Manager) ArchiveResource(ctx context.Context, id string) error {
	_, err := m.db.ExecContext(ctx, "UPDATE resources SET status='archived', archived_at=? WHERE id=?", time.Now().UnixMilli(), id)
	return err
}
