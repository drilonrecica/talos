// SPDX-License-Identifier: AGPL-3.0-only
package dockerapi

import (
	"context"
	"errors"
	"testing"
)

func TestValidateEngineVersion(t *testing.T) {
	tests := []struct {
		value string
		valid bool
	}{
		{"29.5.1", true},
		{"29.5.2", true},
		{"29.6.0", true},
		{"30.0.0", true},
		{"29.5.1-ce", true},
		{"29.5.1+dfsg.1", true},
		{"v29.5.1", true},
		{"29.5.1-rc.1", false},
		{"29.5.1-beta1", false},
		{"29.5.0", false},
		{"29.4.99", false},
		{"28.99.99", false},
		{"", false},
		{"29.5", false},
		{"29.5.x", false},
		{"29.5.1 ", false},
		{"29.5.1+", false},
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			err := ValidateEngineVersion(test.value)
			if (err == nil) != test.valid {
				t.Fatalf("ValidateEngineVersion(%q) error=%v, valid=%v", test.value, err, test.valid)
			}
		})
	}
}

func TestRequireSupportedEngine(t *testing.T) {
	tests := []struct {
		name    string
		version string
		err     error
		valid   bool
	}{
		{name: "minimum", version: "29.5.1", valid: true},
		{name: "newer", version: "30.0.0", valid: true},
		{name: "older", version: "29.5.0"},
		{name: "missing"},
		{name: "probe failure", err: errors.New("daemon unavailable")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &versionClient{Client: noopClient{}, version: Version{EngineVersion: test.version}, err: test.err}
			err := RequireSupportedEngine(context.Background(), client)
			if (err == nil) != test.valid {
				t.Fatalf("RequireSupportedEngine() error=%v, valid=%v", err, test.valid)
			}
		})
	}
}

type versionClient struct {
	Client
	version Version
	err     error
}

func (client *versionClient) Version(context.Context) (Version, error) {
	return client.version, client.err
}

type noopClient struct{}

func (noopClient) List(context.Context) ([]Container, error)        { return nil, nil }
func (noopClient) Inspect(context.Context, string) (Inspect, error) { return Inspect{}, nil }
func (noopClient) Stats(context.Context, string) (Stats, error)     { return Stats{}, nil }
func (noopClient) Events(context.Context) <-chan Event              { return make(chan Event) }
func (noopClient) Version(context.Context) (Version, error)         { return Version{}, nil }
func (noopClient) Diagnostics(context.Context) (Diagnostics, error) { return Diagnostics{}, nil }
