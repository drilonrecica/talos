// SPDX-License-Identifier: AGPL-3.0-only
package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// EnvironmentSecret reads either NAME or NAME_FILE. File values have only a
// trailing line ending removed so spaces in passwords remain significant.
func EnvironmentSecret(name string) (string, error) {
	value, path := os.Getenv(name), os.Getenv(name+"_FILE")
	if value != "" && path != "" {
		return "", fmt.Errorf("configure only one of %s or %s_FILE", name, name)
	}
	if path == "" {
		return value, nil
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s_FILE: %w", name, err)
	}
	return strings.TrimRight(string(contents), "\r\n"), nil
}

// BootstrapAdmin creates the single admin only when the database has no user.
// Once an admin exists, bootstrap inputs are deliberately ignored.
func BootstrapAdmin(ctx context.Context, credentials *Credentials, setup *SetupService) (bool, error) {
	if credentials == nil || credentials.db == nil {
		return false, errors.New("credential repository is unavailable")
	}
	var count int
	if err := credentials.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}
	username, err := EnvironmentSecret("BINNACLE_ADMIN_USERNAME")
	if err != nil {
		return false, err
	}
	password, err := EnvironmentSecret("BINNACLE_ADMIN_PASSWORD")
	if err != nil {
		return false, err
	}
	if username == "" && password == "" {
		return false, nil
	}
	if username == "" || password == "" {
		return false, errors.New("BINNACLE_ADMIN_USERNAME and BINNACLE_ADMIN_PASSWORD must be configured together")
	}
	if _, err = credentials.CreateAdmin(ctx, username, password); err != nil {
		return false, err
	}
	if err = setup.Disable(ctx); err != nil {
		return false, err
	}
	return true, nil
}
