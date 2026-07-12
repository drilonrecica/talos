// SPDX-License-Identifier: AGPL-3.0-only
package storage

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestDeletionJobJSONProgressContract(t *testing.T) {
	encoded, err := json.Marshal(DeletionJob{TotalRows: 10, DeletedRows: 4})
	if err != nil {
		t.Fatal(err)
	}
	var progress map[string]any
	if err := json.Unmarshal(encoded, &progress); err != nil {
		t.Fatal(err)
	}
	if progress["totalRows"] != float64(10) || progress["deletedRows"] != float64(4) {
		t.Fatalf("progress=%v", progress)
	}
}

func TestScopedDeletionPreservesConfigurationAndSupportsRetry(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m := New(filepath.Join(dir, "binnacle.db"), filepath.Join(dir, "run"))
	if err := m.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	now := time.Now().UTC()
	old := now.Add(-2 * time.Hour).UnixMilli()
	fresh := now.UnixMilli()
	if _, err := m.db.ExecContext(ctx, "INSERT INTO users(id,username,password_hash,created_at,updated_at) VALUES('usr_test','admin','hash',?,?)", fresh, fresh); err != nil {
		t.Fatal(err)
	}
	if _, err := m.db.ExecContext(ctx, "INSERT INTO settings(key,value_json,updated_at) VALUES('retention','{}',?)", fresh); err != nil {
		t.Fatal(err)
	}
	if _, err := m.db.ExecContext(ctx, "INSERT INTO host_samples_10s(ts,host_id,cpu_busy_pct) VALUES(?,'host',1),(?,'host',2)", old, fresh); err != nil {
		t.Fatal(err)
	}
	preview, err := m.PreviewDeletion(ctx, DeletionRequest{Kind: DeleteBefore, Before: now.Add(-time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if preview.TotalRows != 1 || preview.Scope.Kind != DeleteBefore {
		t.Fatalf("preview=%+v", preview)
	}
	if _, err = m.CreateDeletion(ctx, preview.Token, "wrong", "usr_test"); err == nil {
		t.Fatal("wrong confirmation accepted")
	}
	job, err := m.CreateDeletion(ctx, preview.Token, preview.Confirmation, "usr_test")
	if err != nil {
		t.Fatal(err)
	}
	if err = m.RunDeletion(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	var samples, users, settings, actor int
	_ = m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM host_samples_10s").Scan(&samples)
	_ = m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&users)
	_ = m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM settings").Scan(&settings)
	_ = m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM history_deletion_jobs WHERE requested_by='usr_test'").Scan(&actor)
	if samples != 1 || users != 1 || settings != 1 || actor != 1 {
		t.Fatalf("samples=%d users=%d settings=%d actor=%d", samples, users, settings, actor)
	}
	second, err := m.PreviewDeletion(ctx, DeletionRequest{Kind: DeleteAll})
	if err != nil {
		t.Fatal(err)
	}
	retry, err := m.CreateDeletion(ctx, second.Token, second.Confirmation, "usr_test")
	if err != nil {
		t.Fatal(err)
	}
	if err = m.CancelDeletion(ctx, retry.ID); err != nil {
		t.Fatal(err)
	}
	if err = m.RetryDeletion(ctx, retry.ID); err != nil {
		t.Fatal(err)
	}
	if err = m.RunDeletion(ctx, retry.ID); err != nil {
		t.Fatal(err)
	}
	completed, err := m.DeletionJob(ctx, retry.ID)
	if err != nil || completed.State != "completed" || completed.DeletedRows != completed.TotalRows {
		t.Fatalf("job=%+v err=%v", completed, err)
	}
}

func TestResourceDeletionAndArchivedPurgeBoundaries(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m := New(filepath.Join(dir, "binnacle.db"), filepath.Join(dir, "run"))
	if err := m.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	now := time.Now().UTC().UnixMilli()
	if _, err := m.db.ExecContext(ctx, "INSERT INTO hosts(id,identity_hash,name,updated_at) VALUES('host','identity','host','now')"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.db.ExecContext(ctx, "INSERT INTO resources(id,host_id,stable_key,source_kind,name,category,status,first_seen_at,last_seen_at) VALUES('res_test','host','stable','compose','test','service','healthy',?,?)", now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := m.db.ExecContext(ctx, "INSERT INTO container_instances(id,resource_id,name,created_at) VALUES('container123','res_test','container',?)", now); err != nil {
		t.Fatal(err)
	}
	insert := func() {
		if _, err := m.db.ExecContext(ctx, "INSERT INTO resource_samples_10s(ts,resource_id,active_instance_count) VALUES(?,'res_test',1)", now); err != nil {
			t.Fatal(err)
		}
		if _, err := m.db.ExecContext(ctx, "INSERT INTO container_instance_samples_10s(ts,container_instance_id) VALUES(?,'container123')", now); err != nil {
			t.Fatal(err)
		}
		if _, err := m.db.ExecContext(ctx, "INSERT INTO events(id,ts,resource_id,type,severity,summary,source,created_at) VALUES(?,?,?,?,?,?,?,?)", "evt_"+newID(t), now, "res_test", "test", "info", "test", "test", now); err != nil {
			t.Fatal(err)
		}
	}
	insert()
	preview, err := m.PreviewDeletion(ctx, DeletionRequest{Kind: DeleteResource, ResourceID: "res_test"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := m.CreateDeletion(ctx, preview.Token, preview.Confirmation, "usr")
	if err != nil {
		t.Fatal(err)
	}
	if err = m.RunDeletion(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	var resources, instances, samples int
	_ = m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM resources").Scan(&resources)
	_ = m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM container_instances").Scan(&instances)
	_ = m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM resource_samples_10s").Scan(&samples)
	if resources != 1 || instances != 1 || samples != 0 {
		t.Fatalf("resource=%d instances=%d samples=%d", resources, instances, samples)
	}
	if _, err = m.db.ExecContext(ctx, "UPDATE resources SET status='archived',archived_at=? WHERE id='res_test'", now); err != nil {
		t.Fatal(err)
	}
	insert()
	preview, err = m.PreviewDeletion(ctx, DeletionRequest{Kind: DeleteArchived, ResourceID: "res_test"})
	if err != nil {
		t.Fatal(err)
	}
	job, err = m.CreateDeletion(ctx, preview.Token, preview.Confirmation, "usr")
	if err != nil {
		t.Fatal(err)
	}
	if err = m.RunDeletion(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	_ = m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM resources").Scan(&resources)
	_ = m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM container_instances").Scan(&instances)
	if resources != 0 || instances != 0 {
		t.Fatalf("archived resource=%d instances=%d", resources, instances)
	}
	if _, err = m.db.ExecContext(ctx, "INSERT INTO resources(id,host_id,stable_key,source_kind,name,category,status,first_seen_at,last_seen_at) VALUES('res_reset','host','reset','compose','reset','service','healthy',?,?)", now, now); err != nil {
		t.Fatal(err)
	}
	reset, err := m.PreviewDeletion(ctx, DeletionRequest{Kind: DeleteAll})
	if err != nil {
		t.Fatal(err)
	}
	resetJob, err := m.CreateDeletion(ctx, reset.Token, reset.Confirmation, "usr")
	if err != nil {
		t.Fatal(err)
	}
	if err = m.RunDeletion(ctx, resetJob.ID); err != nil {
		t.Fatal(err)
	}
	var hosts int
	_ = m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM hosts").Scan(&hosts)
	_ = m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM resources").Scan(&resources)
	if hosts != 0 || resources != 0 {
		t.Fatalf("reset hosts=%d resources=%d", hosts, resources)
	}
}

func TestDeletionJobsConflict(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m := New(filepath.Join(dir, "binnacle.db"), filepath.Join(dir, "run"))
	if err := m.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	one, _ := m.PreviewDeletion(ctx, DeletionRequest{Kind: DeleteAll})
	if _, err := m.CreateDeletion(ctx, one.Token, one.Confirmation, "usr"); err != nil {
		t.Fatal(err)
	}
	two, _ := m.PreviewDeletion(ctx, DeletionRequest{Kind: DeleteAll})
	if _, err := m.CreateDeletion(ctx, two.Token, two.Confirmation, "usr"); err == nil {
		t.Fatal("conflicting job accepted")
	}
}

func TestDeletionJobsRecoverAfterRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m := New(filepath.Join(dir, "binnacle.db"), filepath.Join(dir, "run"))
	if err := m.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	insert := func(id, state string) {
		if _, err := m.db.ExecContext(ctx, "INSERT INTO history_deletion_jobs(id,kind,fence_ts,confirmation,state,requested_at,total_rows) VALUES(?,'all',?,'RESET ALL HISTORY',?,?,0)", id, time.Now().UnixMilli(), state, time.Now().UnixMilli()); err != nil {
			t.Fatal(err)
		}
	}
	insert("job_running", "running")
	if err := m.recoverDeletionJobs(ctx); err != nil {
		t.Fatal(err)
	}
	var running string
	_ = m.db.QueryRowContext(ctx, "SELECT state FROM history_deletion_jobs WHERE id='job_running'").Scan(&running)
	if running != "queued" {
		t.Fatalf("running=%s", running)
	}
	_, _ = m.db.ExecContext(ctx, "UPDATE history_deletion_jobs SET state='completed' WHERE id='job_running'")
	insert("job_cancelling", "cancelling")
	if err := m.recoverDeletionJobs(ctx); err != nil {
		t.Fatal(err)
	}
	var cancelling string
	_ = m.db.QueryRowContext(ctx, "SELECT state FROM history_deletion_jobs WHERE id='job_cancelling'").Scan(&cancelling)
	if cancelling != "cancelled" {
		t.Fatalf("cancelling=%s", cancelling)
	}
}

func newID(t *testing.T) string {
	t.Helper()
	id, err := newDeletionID()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
