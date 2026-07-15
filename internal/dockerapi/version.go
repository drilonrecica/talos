// SPDX-License-Identifier: AGPL-3.0-only
package dockerapi

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	MinimumEngineVersion = "29.5.1"
	versionProbeTimeout  = 5 * time.Second
)

var minimumEngineVersion = semanticVersion{major: 29, minor: 5, patch: 1}

type semanticVersion struct {
	major int
	minor int
	patch int
}

// ValidateEngineVersion rejects daemon releases that do not contain all fixes
// required by Binnacle. Distribution backports under older version strings are
// deliberately not accepted.
func ValidateEngineVersion(value string) error {
	if hasPrereleaseSuffix(value) {
		return fmt.Errorf("Docker Engine prerelease %q is unsupported; version %s or newer is required", value, MinimumEngineVersion)
	}
	version, err := parseEngineVersion(value)
	if err != nil {
		return fmt.Errorf("Docker Engine version %q is invalid: %w", value, err)
	}
	if version.less(minimumEngineVersion) {
		return fmt.Errorf("Docker Engine %s is unsupported; version %s or newer is required", value, MinimumEngineVersion)
	}
	return nil
}

func hasPrereleaseSuffix(value string) bool {
	value = strings.TrimPrefix(value, "v")
	index := strings.IndexByte(value, '-')
	if index < 0 {
		return false
	}
	first, _, _ := strings.Cut(strings.ToLower(value[index+1:]), ".")
	for _, prefix := range []string{"alpha", "beta", "pre", "preview", "rc"} {
		if first == prefix || allDigits(strings.TrimPrefix(first, prefix)) || allDigits(strings.TrimPrefix(first, prefix+"-")) {
			return true
		}
	}
	return false
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

// RequireSupportedEngine performs the startup probe with a fixed upper bound.
func RequireSupportedEngine(ctx context.Context, client Client) error {
	if client == nil {
		return fmt.Errorf("Docker Engine version probe requires a client")
	}
	probeCtx, cancel := context.WithTimeout(ctx, versionProbeTimeout)
	defer cancel()
	version, err := client.Version(probeCtx)
	if err != nil {
		return fmt.Errorf("probe Docker Engine version: %w", err)
	}
	return ValidateEngineVersion(version.EngineVersion)
}

func parseEngineVersion(value string) (semanticVersion, error) {
	if value == "" || strings.TrimSpace(value) != value {
		return semanticVersion{}, fmt.Errorf("expected major.minor.patch")
	}
	value = strings.TrimPrefix(value, "v")
	if index := strings.IndexAny(value, "-+"); index >= 0 {
		if index == len(value)-1 || !validVersionSuffix(value[index+1:]) {
			return semanticVersion{}, fmt.Errorf("invalid suffix")
		}
		value = value[:index]
	}
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return semanticVersion{}, fmt.Errorf("expected major.minor.patch")
	}
	numbers := make([]int, 3)
	for index, part := range parts {
		if part == "" {
			return semanticVersion{}, fmt.Errorf("expected numeric version component")
		}
		parsed, err := strconv.Atoi(part)
		if err != nil || parsed < 0 {
			return semanticVersion{}, fmt.Errorf("expected numeric version component")
		}
		numbers[index] = parsed
	}
	return semanticVersion{major: numbers[0], minor: numbers[1], patch: numbers[2]}, nil
}

func validVersionSuffix(value string) bool {
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '.' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func (version semanticVersion) less(other semanticVersion) bool {
	if version.major != other.major {
		return version.major < other.major
	}
	if version.minor != other.minor {
		return version.minor < other.minor
	}
	return version.patch < other.patch
}
