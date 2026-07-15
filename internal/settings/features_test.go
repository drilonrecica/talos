// SPDX-License-Identifier: AGPL-3.0-only

package settings

import (
	"strings"
	"testing"
)

func TestFeatureFlagsDefaultDisabledAndLoadFromEnvironment(t *testing.T) {
	defaults := Defaults()
	if defaults.Features.AdvancedAuth || defaults.Features.Portability {
		t.Fatalf("feature defaults=%+v", defaults.Features)
	}

	values := map[string]string{
		"BINNACLE_FEATURE_ADVANCED_AUTH": "true",
		"BINNACLE_FEATURE_PORTABILITY":   "true",
		"BINNACLE_PROMETHEUS_ENABLED":    "true",
	}
	config, effective, err := LoadWith(func(key string) string { return values[key] }, func(string) bool { return false }, NoopOverrideProvider{})
	if err != nil {
		t.Fatal(err)
	}
	if !config.Features.AdvancedAuth || !config.Features.Portability || !config.Prometheus.Enabled {
		t.Fatalf("loaded config=%+v", config)
	}
	if effective["features.portability"].Source != SourceEnvironment {
		t.Fatalf("effective portability=%+v", effective["features.portability"])
	}
}

func TestFeatureFlagCombinationValidation(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
		want   string
	}{
		{
			name: "proxy authentication",
			values: map[string]string{
				"BINNACLE_AUTH_MODE":            "proxy",
				"BINNACLE_AUTH_PROXY_CIDRS":     "10.0.0.1/32",
				"BINNACLE_AUTH_ALLOWED_SUBJECT": "admin@example.test",
			},
			want: "features.advanced_auth=true",
		},
		{
			name:   "prometheus",
			values: map[string]string{"BINNACLE_PROMETHEUS_ENABLED": "true"},
			want:   "features.portability=true",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := LoadWith(func(key string) string { return test.values[key] }, func(string) bool { return false }, NoopOverrideProvider{})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err=%v, want containing %q", err, test.want)
			}
		})
	}
}
