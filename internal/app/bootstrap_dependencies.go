//go:build bootstrapdeps

// SPDX-License-Identifier: AGPL-3.0-only

// This manifest keeps the runtime dependencies selected during repository
// bootstrap reproducible until their owning packages import them directly:
// configuration (T008), storage (T010), Docker access (T035), and auth (T073).
// The bootstrapdeps tag is never used by production builds.
package app

import (
	_ "github.com/moby/moby/client"
	_ "golang.org/x/crypto/argon2"
)
