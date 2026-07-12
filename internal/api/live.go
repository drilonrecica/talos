// SPDX-License-Identifier: AGPL-3.0-only
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	authpkg "github.com/drilonrecica/binnacle/internal/auth"
	"github.com/drilonrecica/binnacle/internal/metrics"
)

type Authorizer interface{ Authorize(*http.Request) bool }
type DemoAuthorizer bool

func (a DemoAuthorizer) Authorize(*http.Request) bool { return bool(a) }
func (s *Server) EnableLive(engine *metrics.Engine, auth Authorizer, protection *authpkg.Protection) {
	s.Handle("/api/v1/live", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			WriteError(w, 405, Error{Code: "method_not_allowed", Message: "Only GET is supported."})
			return
		}
		if auth == nil || !auth.Authorize(r) {
			WriteError(w, 401, Error{Code: "unauthorized", Message: "Authentication is required."})
			return
		}
		if ok, retry := protection.AllowLive(r); !ok {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", maxRetry(retry)))
			WriteError(w, 429, Error{Code: "rate_limited", Message: "Too many live connections. Try again shortly.", Details: map[string]int{"retryAfterSeconds": maxRetry(retry)}})
			return
		}
		stream(w, r, engine)
	}))
	s.Handle("/api/v1/session", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth == nil || !auth.Authorize(r) {
			WriteError(w, 401, Error{Code: "unauthorized", Message: "Authentication is required."})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
}
func stream(w http.ResponseWriter, r *http.Request, e *metrics.Engine) {
	f, ok := w.(http.Flusher)
	if !ok {
		WriteError(w, 500, Error{Code: "stream_unsupported", Message: "Streaming is unavailable."})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	sub := e.Subscribe()
	defer sub.Close()
	if n, err := strconv.ParseUint(r.Header.Get("Last-Event-ID"), 10, 64); err == nil {
		for _, event := range e.EventsAfter(metrics.Sequence(n)) {
			writeFrame(w, "event", event.ID, event)
			f.Flush()
		}
	}
	beat := time.NewTicker(20 * time.Second)
	defer beat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-beat.C:
			fmt.Fprint(w, ": heartbeat\n\n")
			f.Flush()
		case message, ok := <-sub.C:
			if !ok {
				return
			}
			if message.Snapshot != nil {
				writeFrame(w, "snapshot", message.Snapshot.Sequence, message.Snapshot)
			} else if message.Event != nil {
				writeFrame(w, "event", message.Event.ID, message.Event)
			}
			f.Flush()
		}
	}
}
func writeFrame(w http.ResponseWriter, event string, id metrics.Sequence, value any) {
	data, _ := json.Marshal(value)
	fmt.Fprintf(w, "event: %s\nid: %d\ndata: %s\n\n", event, id, data)
}
