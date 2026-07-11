// SPDX-License-Identifier: AGPL-3.0-only
package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

func HostIdentity(machineID, installationID string) string {
	v := machineID
	if v == "" {
		v = installationID
	}
	s := sha256.Sum256([]byte(v))
	return hex.EncodeToString(s[:])
}
func (m *Manager) UpsertHost(ctx context.Context, id, identity, name string) error {
	_, e := m.db.ExecContext(ctx, "INSERT INTO hosts(id,identity_hash,name,updated_at) VALUES(?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,updated_at=excluded.updated_at", id, identity, name, time.Now().UTC().Format(time.RFC3339Nano))
	return e
}
func (m *Manager) OpenBoot(ctx context.Context, hostID, boot string) error {
	if m.db == nil {
		return fmt.Errorf("storage is not open")
	}
	_, e := m.db.ExecContext(ctx, "UPDATE boot_sessions SET ended_at=? WHERE host_id=? AND ended_at IS NULL AND boot_identity<>?", time.Now().UTC().Format(time.RFC3339Nano), hostID, boot)
	if e != nil {
		return e
	}
	_, e = m.db.ExecContext(ctx, "INSERT OR IGNORE INTO boot_sessions(host_id,boot_identity,started_at) VALUES(?,?,?)", hostID, boot, time.Now().UTC().Format(time.RFC3339Nano))
	return e
}
