// SPDX-License-Identifier: AGPL-3.0-only
package settings

import (
	"strings"
	"testing"
)

func TestBinnacleConfigurationNames(t *testing.T) {
	config := Defaults()
	config.Normalize()
	if config.Paths.DataDir != "/var/lib/binnacle" {
		t.Fatalf("data directory = %q", config.Paths.DataDir)
	}
	if config.Paths.DatabasePath != "/var/lib/binnacle/binnacle.db" {
		t.Fatalf("database path = %q", config.Paths.DatabasePath)
	}

	for name := range environment {
		if !strings.HasPrefix(name, "BINNACLE_") {
			t.Fatalf("environment variable %q has the wrong prefix", name)
		}
	}

	const explicit = "/tmp/binnacle.toml"
	path := Discover(func(name string) string {
		if name == "BINNACLE_CONFIG_FILE" {
			return explicit
		}
		return ""
	}, func(string) bool { return false })
	if path != explicit {
		t.Fatalf("discovered config path = %q", path)
	}
}
