// SPDX-License-Identifier: AGPL-3.0-only
package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/drilonrecica/binnacle/internal/metrics"
)

func TestWriteBatchPersistsContainerInstancesAndSamples(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	manager := New(filepath.Join(dir, "binnacle.db"), filepath.Join(dir, "run"))
	if err := manager.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	now := time.Now().UTC()
	if err := manager.UpsertHost(ctx, "host", "host-identity", "test-host"); err != nil {
		t.Fatal(err)
	}
	if err := manager.UpsertResource(ctx, Resource{ID: "res_web", HostID: "host", StableKey: "web", SourceKind: "compose", Name: "web", Category: "service", Status: "healthy"}); err != nil {
		t.Fatal(err)
	}
	cpu := 10.0
	mem := int64(128 * 1024 * 1024)
	pids := uint64(12)
	resource := metrics.ResourceSnapshot{
		ID:   metrics.ResourceID("res_web"),
		Name: "web",
		Components: []metrics.ResourceComponent{
			{ID: "c0ffee123456", Name: "/web_1", Status: metrics.StatusHealthy, CPUHostPercent: &cpu, MemoryBytes: &mem, PIDs: &pids},
		},
	}
	snap := metrics.Snapshot{Sequence: 1, At: now, BootIdentity: "boot-1", Host: metrics.HostObservation{At: now}, Resources: []metrics.ResourceSnapshot{resource}}
	if err := manager.WriteBatch(ctx, metrics.PersistenceBatch{Snapshot: snap}); err != nil {
		t.Fatal(err)
	}

	var instanceCount int
	if err := manager.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM container_instances WHERE id=?", "c0ffee123456").Scan(&instanceCount); err != nil {
		t.Fatal(err)
	}
	if instanceCount != 1 {
		t.Fatalf("expected 1 container instance, got %d", instanceCount)
	}

	var sampleCount int
	if err := manager.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM container_instance_samples_10s WHERE container_instance_id=?", "c0ffee123456").Scan(&sampleCount); err != nil {
		t.Fatal(err)
	}
	if sampleCount != 1 {
		t.Fatalf("expected 1 container instance sample, got %d", sampleCount)
	}
}

func TestWriteBatchPersistsEventExtendedFields(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	manager := New(filepath.Join(dir, "binnacle.db"), filepath.Join(dir, "run"))
	if err := manager.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	now := time.Now().UTC()
	event := metrics.Event{
		ID:                1,
		At:                now,
		Type:              "container_oom",
		ResourceID:        "res_web",
		ContainerInstance: "c0ffee123456",
		Severity:          "critical",
		Message:           "container ran out of memory",
		Details:           `{"oom_score":1000}`,
		CorrelationKey:    "c0ffee123456",
	}
	snap := metrics.Snapshot{Sequence: 1, At: now, BootIdentity: "boot-1", Host: metrics.HostObservation{At: now}}
	if err := manager.WriteBatch(ctx, metrics.PersistenceBatch{Snapshot: snap, Events: []metrics.Event{event}}); err != nil {
		t.Fatal(err)
	}

	var severity, details, correlation, container string
	row := manager.DB().QueryRowContext(ctx, "SELECT severity, details_json, correlation_key, container_instance_id FROM events WHERE id=?", "1")
	if err := row.Scan(&severity, &details, &correlation, &container); err != nil {
		t.Fatal(err)
	}
	if severity != "critical" {
		t.Fatalf("expected severity critical, got %s", severity)
	}
	if details != `{"oom_score":1000}` {
		t.Fatalf("expected details, got %s", details)
	}
	if correlation != "c0ffee123456" {
		t.Fatalf("expected correlation key, got %s", correlation)
	}
	if container != "c0ffee123456" {
		t.Fatalf("expected container instance id, got %s", container)
	}
}
