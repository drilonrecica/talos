// SPDX-License-Identifier: AGPL-3.0-only
package auth_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/drilonrecica/binnacle/internal/auth"
	"github.com/drilonrecica/binnacle/internal/storage"
)

func TestSingleAdminCredentialLifecycle(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db := storage.New(filepath.Join(dir, "binnacle.db"), filepath.Join(dir, "run"))
	if err := db.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	credentials := auth.NewCredentials(db.DB())
	created, err := credentials.CreateAdmin(ctx, "Admin", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if created.Username != "admin" {
		t.Fatalf("username=%q", created.Username)
	}
	if _, err = credentials.CreateAdmin(ctx, "second", "another secure password"); !errors.Is(err, auth.ErrAdminExists) {
		t.Fatalf("duplicate err=%v", err)
	}
	verified, err := credentials.Authenticate(ctx, "admin", "correct horse battery staple")
	if err != nil || verified.ID != created.ID {
		t.Fatalf("verified=%+v err=%v", verified, err)
	}
	if _, err = credentials.Authenticate(ctx, "missing", "wrong password value"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("missing user err=%v", err)
	}
	if _, err = credentials.Authenticate(ctx, "@invalid", "wrong password value"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("invalid username err=%v", err)
	}
	if _, err = credentials.Authenticate(ctx, "admin", "wrong password value"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("wrong password err=%v", err)
	}
}

func TestPasswordPolicyBoundaries(t *testing.T) {
	for _, password := range []string{strings.Repeat("a", 12), strings.Repeat("é", 128)} {
		if err := auth.ValidatePassword(password); err != nil {
			t.Fatalf("valid boundary: %v", err)
		}
	}
	for _, password := range []string{strings.Repeat("a", 11), strings.Repeat("a", 129)} {
		if err := auth.ValidatePassword(password); err == nil {
			t.Fatalf("accepted length %d", len(password))
		}
	}
}
