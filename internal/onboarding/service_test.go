// SPDX-License-Identifier: AGPL-3.0-only
package onboarding

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/drilonrecica/binnacle/internal/diagnostics"
	"github.com/drilonrecica/binnacle/internal/storage"
)

func TestOnboardingPersistsAndCompletesDespiteDiagnosticFailure(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	manager := storage.New(filepath.Join(dir, "binnacle.db"), filepath.Join(dir, "run"))
	if err := manager.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	checker := diagnostics.OnboardingChecker{
		HostProc: "/missing", HostSys: "/missing", DataDir: dir, DB: manager.DB(),
		ReadFile: func(string) ([]byte, error) { return nil, errors.New("unavailable") },
	}
	service := New(manager.DB(), checker)
	if _, err := service.Update(ctx, "public", "balanced"); err != nil {
		t.Fatal(err)
	}
	state, err := service.Diagnose(ctx, false)
	if err != nil || len(state.Diagnostics) != 7 {
		t.Fatalf("state=%+v err=%v", state, err)
	}
	state, err = service.Complete(ctx)
	if err != nil || state.CompletedAt == nil {
		t.Fatalf("state=%+v err=%v", state, err)
	}
	if err = service.DismissChecklist(ctx); err != nil {
		t.Fatal(err)
	}
	state, err = service.State(ctx)
	if err != nil || !state.ChecklistDismissed {
		t.Fatalf("state=%+v err=%v", state, err)
	}
}
