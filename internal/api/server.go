// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

const MaxRequestBodyBytes int64 = 1 << 20

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}
type errorEnvelope struct {
	Error Error `json:"error"`
}
type Server struct {
	mux    *http.ServeMux
	next   atomic.Uint64
	logger *slog.Logger
}

func New() *Server {
	s := &Server{mux: http.NewServeMux(), logger: slog.Default()}
	s.mux.HandleFunc("/api/v1/", s.notFound)
	return s
}
func (s *Server) SetLogger(l *slog.Logger)                    { s.logger = l }
func (s *Server) Handle(pattern string, handler http.Handler) { s.mux.Handle(pattern, s.wrap(handler)) }
func (s *Server) Handler() http.Handler                       { return s.wrap(s.mux) }
func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusNotFound, Error{Code: "not_found", Message: "The requested endpoint does not exist."})
}
func (s *Server) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := fmt.Sprintf("req_%d", s.next.Add(1))
		w.Header().Set("X-Request-ID", id)
		setSecurityHeaders(w, r)
		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if recover() != nil {
				WriteError(w, http.StatusInternalServerError, Error{Code: "internal_error", Message: "The server could not process the request."})
			}
		}()
		next.ServeHTTP(rec, r)
		s.logger.Info("api request",
			slog.String("id", id),
			slog.String("method", r.Method),
			slog.String("path", sanitizePath(r.URL)),
			slog.Int("status", rec.status),
			slog.String("client", clientPrefix(r)),
		)
	})
}

func setSecurityHeaders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none';")
	if r.TLS != nil {
		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
	}
}

func sanitizePath(u *url.URL) string {
	values := u.Query()
	redact := []string{"token", "setup_token", "password", "csrf"}
	for _, key := range redact {
		if values.Has(key) {
			values.Set(key, "[REDACTED]")
		}
	}
	u = &url.URL{Path: u.Path, RawQuery: values.Encode()}
	return u.RequestURI()
}

func clientPrefix(r *http.Request) string {
	host, _, _ := strings.Cut(r.RemoteAddr, ":")
	if host == "" {
		return "unknown"
	}
	return host
}

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
func (r *responseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func WriteError(w http.ResponseWriter, status int, e Error) {
	WriteJSON(w, status, errorEnvelope{Error: e})
}
func DecodeJSON(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, MaxRequestBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if dec.More() {
		return fmt.Errorf("request body must contain one JSON value")
	}
	return nil
}
func UTC(t time.Time) time.Time               { return t.UTC() }
func Context(r *http.Request) context.Context { return r.Context() }
