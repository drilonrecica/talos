// SPDX-License-Identifier: AGPL-3.0-only

package alerts_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/drilonrecica/binnacle/internal/alerts"
	"github.com/drilonrecica/binnacle/internal/storage"
)

func TestDefaultRuleSeedingIsIdempotent(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := storage.New(filepath.Join(dir, "binnacle.db"), filepath.Join(dir, "runtime"))
	if err := store.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	repo := alerts.NewRepository(store.DB())
	if err := repo.SeedDefaults(ctx); err != nil {
		t.Fatal(err)
	}
	if err := repo.SeedDefaults(ctx); err != nil {
		t.Fatal(err)
	}
	rules, err := repo.Rules(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(rules), len(alerts.DefaultRules()); got != want {
		t.Fatalf("rules=%d want %d", got, want)
	}
	for _, rule := range rules {
		if !rule.BuiltIn || !rule.Enabled {
			t.Fatalf("default rule not enabled and built-in: %+v", rule)
		}
	}
}

func TestChecksAlertsMigrationIsCurrent(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := storage.New(filepath.Join(dir, "binnacle.db"), filepath.Join(dir, "runtime"))
	if err := store.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	version, err := store.SchemaVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if version != 18 {
		t.Fatalf("schema version=%d want 18", version)
	}
}
