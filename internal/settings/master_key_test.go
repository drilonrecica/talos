// SPDX-License-Identifier: AGPL-3.0-only
package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMasterKeyFileLoadsWithoutExposingContents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "master-key")
	key := strings.Repeat("ab", 32)
	if err := os.WriteFile(path, []byte(key+"\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config, values, err := LoadWith(func(name string) string {
		if name == "BINNACLE_MASTER_KEY_FILE" {
			return path
		}
		return ""
	}, func(string) bool { return false }, NoopOverrideProvider{})
	if err != nil {
		t.Fatal(err)
	}
	if config.Paths.MasterKey != key || config.Paths.MasterKeyFile != path {
		t.Fatalf("master key file was not resolved")
	}
	if value := values["paths.master_key"]; !value.Secret || value.Value != "configured" || value.Source != SourceEnvironment {
		t.Fatalf("effective master key=%+v", value)
	}
	if values["paths.master_key_file"].Value != path {
		t.Fatalf("effective master key file=%+v", values["paths.master_key_file"])
	}
}

func TestMasterKeyFileFailsClosed(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "valid")
	empty := filepath.Join(dir, "empty")
	large := filepath.Join(dir, "large")
	for path, content := range map[string][]byte{
		valid: []byte(strings.Repeat("a", 64)),
		empty: nil,
		large: []byte(strings.Repeat("a", 4097)),
	} {
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name, direct, file, want string
	}{
		{"both configured", strings.Repeat("b", 64), valid, "cannot both"},
		{"relative path", "", "master-key", "absolute path"},
		{"missing file", "", filepath.Join(dir, "missing"), "read master key file"},
		{"non-regular file", "", dir, "regular file"},
		{"empty file", "", empty, "is empty"},
		{"large file", "", large, "too large"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			values := map[string]string{"BINNACLE_MASTER_KEY": test.direct, "BINNACLE_MASTER_KEY_FILE": test.file}
			_, _, err := LoadWith(func(name string) string { return values[name] }, func(string) bool { return false }, NoopOverrideProvider{})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err=%v, want containing %q", err, test.want)
			}
		})
	}
}

func TestProxyCIDRsRequireExactHosts(t *testing.T) {
	tests := []struct {
		name, httpCIDR, authCIDR string
		wantErr                  bool
	}{
		{"IPv4 hosts", "192.0.2.10/32", "198.51.100.4/32", false},
		{"IPv6 hosts", "2001:db8::10/128", "2001:db8::20/128", false},
		{"broad HTTP network", "10.0.0.0/8", "198.51.100.4/32", true},
		{"broad auth network", "192.0.2.10/32", "10.0.0.0/8", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := Defaults()
			config.HTTP.TrustedProxyCIDRs = []string{test.httpCIDR}
			config.Auth = Auth{Mode: "proxy", ProxyCIDRs: []string{test.authCIDR}, IdentityHeader: "X-Auth-Subject", AllowedSubject: "admin"}
			config.Features.AdvancedAuth = true
			err := config.Validate()
			if (err != nil) != test.wantErr {
				t.Fatalf("Validate() error=%v, wantErr=%v", err, test.wantErr)
			}
		})
	}
}
