// SPDX-License-Identifier: AGPL-3.0-only

package app

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"sync"
)

// HTTPServer exposes the unauthenticated process health endpoint.
type HTTPServer struct {
	address  string
	version  string
	app      *Application
	server   *http.Server
	listener net.Listener
	mu       sync.Mutex
}

func NewHTTPServer(address, version string, application *Application) *HTTPServer {
	h := &HTTPServer{address: address, version: version, app: application}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.health)
	h.server = &http.Server{Addr: address, Handler: mux}
	return h
}

func (h *HTTPServer) Start(context.Context) error {
	ln, err := net.Listen("tcp", h.address)
	if err != nil {
		return err
	}
	h.mu.Lock()
	h.listener = ln
	h.mu.Unlock()
	go func() { _ = h.server.Serve(ln) }()
	return nil
}

func (h *HTTPServer) Stop(ctx context.Context) error { return h.server.Shutdown(ctx) }

func (h *HTTPServer) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	state := h.app.State()
	status := http.StatusServiceUnavailable
	if state == StateRunning {
		status = http.StatusOK
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_ = json.NewEncoder(w).Encode(struct {
			Version string `json:"version"`
			State   State  `json:"state"`
		}{h.version, state})
	}
}
