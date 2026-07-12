// SPDX-License-Identifier: AGPL-3.0-only
package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/drilonrecica/binnacle/internal/metrics"
)

func TestPersistenceSchedulesCurrentSnapshot(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	manager := New(filepath.Join(dir, "binnacle.db"), filepath.Join(dir, "run"))
	if err := manager.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	engine := metrics.NewEngine(8)
	value := 12.5
	at := time.Now().UTC()
	engine.Publish(metrics.Snapshot{At: at, Host: metrics.HostObservation{At: at, CPUPercent: &value}})
	worker := NewPersistence(engine, manager, 10*time.Millisecond, 2)
	if err := worker.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer worker.Stop(ctx)
	deadline := time.Now().Add(time.Second)
	for {
		var count int
		if err := manager.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM host_samples_10s").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("snapshot was not persisted")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
