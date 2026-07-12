// SPDX-License-Identifier: AGPL-3.0-only
package settings

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/drilonrecica/binnacle/internal/storage"
)

func TestSettingsPatchPrecedenceValidationAuditAndConflict(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	manager := storage.New(filepath.Join(dir, "binnacle.db"), filepath.Join(dir, "run"))
	if err := manager.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	base := Defaults()
	effective := effective(base, map[string]Source{"collection.host_interval": SourceEnvironment})
	var applied Config
	service := NewService(NewStore(manager.DB()), base, effective, func(config Config) { applied = config })
	initial, err := service.Snapshot(ctx)
	if err != nil || initial.Revision != 0 || initial.Values["collection.host_interval"].Source != SourceEnvironment {
		t.Fatalf("initial=%+v err=%v", initial, err)
	}
	updated, err := service.Patch(ctx, 0, map[string]string{"collection.host_interval": "3s"}, "usr_admin")
	if err != nil || updated.Revision != 1 || updated.Values["collection.host_interval"].Source != SourceAdmin || applied.Collection.HostInterval.String() != "3s" {
		t.Fatalf("updated=%+v applied=%+v err=%v", updated, applied, err)
	}
	if _, err = service.Patch(ctx, 0, map[string]string{"collection.host_interval": "4s"}, "usr_admin"); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("conflict err=%v", err)
	}
	if _, err = service.Patch(ctx, 1, map[string]string{"paths.data_dir": "/tmp"}, "usr_admin"); err == nil {
		t.Fatal("critical setting accepted")
	}
	if _, err = service.Patch(ctx, 1, map[string]string{"collection.host_interval": "100ms"}, "usr_admin"); err == nil {
		t.Fatal("unsafe interval accepted")
	}
	var auditActor string
	if err = manager.DB().QueryRowContext(ctx, "SELECT actor FROM settings_audit WHERE revision=1").Scan(&auditActor); err != nil || auditActor != "usr_admin" {
		t.Fatalf("actor=%q err=%v", auditActor, err)
	}
}
