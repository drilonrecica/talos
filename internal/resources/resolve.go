// SPDX-License-Identifier: AGPL-3.0-only
package resources

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type Identity struct{ StableKey, Name, Source, Project, Service string }

var categories = map[string]bool{"application": true, "service": true, "database": true, "cache": true, "worker": true, "proxy": true, "infrastructure": true, "unmanaged": true}

func ValidCategory(v string) bool { return categories[strings.ToLower(v)] }
func Resolve(labels map[string]string, fallback, manual string) Identity {
	if manual != "" {
		return Identity{StableKey: "manual:" + safe(manual), Name: fallback, Source: "manual"}
	}
	if id := strings.TrimSpace(labels["coolify.resource.uuid"]); id != "" {
		return Identity{StableKey: "coolify:" + safe(id), Name: fallback, Source: "coolify"}
	}
	return Compose(labels, fallback)
}

func Compose(labels map[string]string, fallback string) Identity {
	p, s := strings.TrimSpace(labels["com.docker.compose.project"]), strings.TrimSpace(labels["com.docker.compose.service"])
	if p != "" && s != "" {
		return Identity{StableKey: "compose:" + safe(p) + ":" + safe(s), Name: s, Source: "compose", Project: p, Service: s}
	}
	return Derived(labels, fallback)
}
func Derived(labels map[string]string, fallback string) Identity {
	h := sha256.New()
	for _, k := range []string{"com.docker.compose.project", "com.docker.compose.service", "org.opencontainers.image.source"} {
		h.Write([]byte(labels[k]))
		h.Write([]byte{0})
	}
	h.Write([]byte(fallback))
	return Identity{StableKey: "derived:" + hex.EncodeToString(h.Sum(nil))[:20], Name: fallback, Source: "derived"}
}
func safe(v string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, v)
}
