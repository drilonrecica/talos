// SPDX-License-Identifier: AGPL-3.0-only

package webembed

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed dist/* dist/assets/*
var files embed.FS

func Handler() http.Handler {
	assets, err := fs.Sub(files, "dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" || uiRoute(name) {
			name = "index.html"
		}
		if strings.Contains(name, "/") || strings.Contains(name, ".") {
			if strings.Contains(name, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
		}
		if name == "index.html" {
			w.Header().Set("Cache-Control", "no-cache")
		}
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/" + name
		fileServer.ServeHTTP(w, r2)
	})
}
func uiRoute(name string) bool {
	switch name {
	case "overview", "resources", "server", "events", "checks", "settings", "login", "setup":
		return true
	}
	return false
}
