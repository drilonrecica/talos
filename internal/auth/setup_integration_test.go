// SPDX-License-Identifier: AGPL-3.0-only
package auth

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/drilonrecica/binnacle/internal/storage"
)

func setupDatabase(t *testing.T) (*storage.Manager, *SetupService) {
	t.Helper()
	dir := t.TempDir()
	manager := storage.New(filepath.Join(dir, "binnacle.db"), filepath.Join(dir, "run"))
	if err := manager.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Close() })
	return manager, NewSetupService(manager.DB())
}

func TestSetupPolicyAndPermanentDisable(t *testing.T) {
	ctx := context.Background()
	_, setup := setupDatabase(t)
	if _, err := setup.Initialize(ctx, ":8080", ""); err == nil {
		t.Fatal("public listener accepted setup without an operator token")
	}
	token := "0123456789abcdefghijklmnopqrstuvwxyz-SETUP"
	if generated, err := setup.Initialize(ctx, ":8080", token); err != nil || generated != "" {
		t.Fatalf("initialize generated=%q err=%v", generated, err)
	}
	if _, err := setup.Claim(ctx, token, "admin", "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	if setup.Available(ctx) {
		t.Fatal("setup remained available after claim")
	}
	if err := setup.Verify(ctx, token); !errors.Is(err, ErrSetupToken) {
		t.Fatalf("replay err=%v", err)
	}
	if _, err := setup.Initialize(ctx, ":8080", token); err != nil {
		t.Fatal(err)
	}
	if setup.Available(ctx) {
		t.Fatal("startup silently re-enabled setup")
	}
}

func TestLocalGeneratedSetupTokenExpires(t *testing.T) {
	ctx := context.Background()
	_, setup := setupDatabase(t)
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	setup.now = func() time.Time { return now }
	generated, err := setup.Initialize(ctx, "127.0.0.1:8080", "")
	if err != nil || len(generated) < 32 {
		t.Fatalf("generated=%q err=%v", generated, err)
	}
	now = now.Add(24 * time.Hour)
	if setup.Available(ctx) || setup.Verify(ctx, generated) == nil {
		t.Fatal("expired setup token accepted")
	}
}

func TestConcurrentSetupClaimAllowsOneAdmin(t *testing.T) {
	ctx := context.Background()
	_, setup := setupDatabase(t)
	token := "0123456789abcdefghijklmnopqrstuvwxyz-RACE"
	if _, err := setup.Initialize(ctx, ":8080", token); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, username := range []string{"admin-one", "admin-two"} {
		wg.Add(1)
		go func(username string) {
			defer wg.Done()
			_, err := setup.Claim(ctx, token, username, "correct horse battery staple")
			errs <- err
		}(username)
	}
	wg.Wait()
	close(errs)
	successes := 0
	for err := range errs {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successful claims=%d", successes)
	}
}
