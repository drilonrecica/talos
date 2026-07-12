// SPDX-License-Identifier: AGPL-3.0-only
package auth

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestBootstrapAdminIsIdempotentAndDisablesSetup(t *testing.T) {
	ctx := context.Background()
	manager, setup := setupDatabase(t)
	credentials := NewCredentials(manager.DB())
	t.Setenv("BINNACLE_ADMIN_USERNAME", "admin")
	t.Setenv("BINNACLE_ADMIN_PASSWORD", "correct horse battery staple")
	created, err := BootstrapAdmin(ctx, credentials, setup)
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	created, err = BootstrapAdmin(ctx, credentials, setup)
	if err != nil || created {
		t.Fatalf("second created=%v err=%v", created, err)
	}
	if setup.Available(ctx) {
		t.Fatal("bootstrap left setup available")
	}
}

func TestEnvironmentSecretFileAndConflict(t *testing.T) {
	path := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(path, []byte("file secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_SECRET_FILE", path)
	value, err := EnvironmentSecret("TEST_SECRET")
	if err != nil || value != "file secret" {
		t.Fatalf("value=%q err=%v", value, err)
	}
	t.Setenv("TEST_SECRET", "direct")
	if _, err = EnvironmentSecret("TEST_SECRET"); err == nil {
		t.Fatal("accepted ambiguous secret sources")
	}
}

func TestEncryptedSecretRoundTripAndRedactedPersistence(t *testing.T) {
	ctx := context.Background()
	manager, _ := setupDatabase(t)
	key := "0123456789abcdef0123456789abcdef"
	store, err := NewSecretStore(manager.DB(), key)
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("https://user:token@example.test/hook")
	if err = store.Put(ctx, "integration.webhook", plaintext); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx, "integration.webhook")
	if err != nil || !bytes.Equal(got, plaintext) {
		t.Fatalf("got=%q err=%v", got, err)
	}
	var ciphertext []byte
	var algorithm string
	var version int
	if err = manager.DB().QueryRowContext(ctx, "SELECT ciphertext,algorithm,key_version FROM encrypted_secrets WHERE key=?", "integration.webhook").Scan(&ciphertext, &algorithm, &version); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ciphertext, plaintext) || algorithm != SecretAlgorithm || version != SecretKeyVersion {
		t.Fatal("plaintext or invalid encryption metadata persisted")
	}
	wrong, _ := NewSecretStore(manager.DB(), "abcdef0123456789abcdef0123456789")
	if _, err = wrong.Get(ctx, "integration.webhook"); err == nil {
		t.Fatal("wrong master key decrypted secret")
	}
	missing, _ := NewSecretStore(manager.DB(), "")
	if err = missing.Put(ctx, "other", []byte("secret")); !errors.Is(err, ErrMasterKeyMissing) {
		t.Fatalf("missing key err=%v", err)
	}
	status, err := missing.Status(ctx, "integration.webhook")
	if err != nil || !status.Configured {
		t.Fatalf("status=%+v err=%v", status, err)
	}
}
