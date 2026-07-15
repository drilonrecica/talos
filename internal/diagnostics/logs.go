// SPDX-License-Identifier: AGPL-3.0-only
package diagnostics

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/drilonrecica/binnacle/internal/dockerapi"
)

const (
	DefaultLogLines   = 500
	MaxLogComponents  = 32
	MaxFollowDuration = 30 * time.Minute
)

type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Component string    `json:"component"`
	Stream    string    `json:"stream"`
	Severity  string    `json:"severity"`
	Message   string    `json:"message"`
}
type LogResult struct {
	Entries   []LogEntry `json:"entries"`
	Truncated bool       `json:"truncated"`
	Redaction string     `json:"redaction"`
}
type LogRequest struct {
	Components []string
	From, To   time.Time
	Limit      int
	Search     string
	Follow     bool
}
type LogService struct {
	Docker    dockerapi.LogClient
	MaxLines  int
	MaxBytes  int64
	redactors []redactor
}

type redactor struct {
	pattern     *regexp.Regexp
	replacement string
}

func NewLogService(client dockerapi.LogClient, maxLines int, maxBytes int64, custom []string) (*LogService, error) {
	if maxLines < 1 || maxLines > 5000 {
		return nil, errors.New("log line limit must be between 1 and 5000")
	}
	if maxBytes < 1 || maxBytes > 1<<20 {
		return nil, errors.New("log byte limit must be between 1 and 1048576")
	}
	if len(custom) > 16 {
		return nil, errors.New("at most 16 custom redaction patterns are allowed")
	}
	keyPrefix := `(?i)((?:"?(?:password|passwd|pwd|token|api[_-]?key|secret)"?)\s*[:=]\s*)`
	patterns := []struct {
		expression, replacement string
	}{
		{`(?i)(authorization\s*[:=]\s*(?:bearer|basic)\s+)[^\s]+`, `${1}[REDACTED]`},
		{keyPrefix + `"(?:\\.|[^"\\])*"`, `${1}"[REDACTED]"`},
		{keyPrefix + `'(?:\\.|[^'\\])*'`, `${1}'[REDACTED]'`},
		{keyPrefix + `[^\s"',;}][^"',;}]*`, `${1}[REDACTED]`},
		{`(?i)([a-z][a-z0-9+.-]*://[^:/\s]+:)[^@/\s]+(@)`, `${1}[REDACTED]${2}`},
		{`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`, `[REDACTED PRIVATE KEY]`},
	}
	compiled := make([]redactor, 0, len(patterns)+len(custom))
	for _, pattern := range patterns {
		r, err := regexp.Compile(pattern.expression)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, redactor{pattern: r, replacement: pattern.replacement})
	}
	for _, expression := range custom {
		r, err := regexp.Compile(expression)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, redactor{pattern: r, replacement: "[REDACTED]"})
	}
	return &LogService{Docker: client, MaxLines: maxLines, MaxBytes: maxBytes, redactors: compiled}, nil
}

func (s *LogService) Redact(message string) string {
	for _, r := range s.redactors {
		message = r.pattern.ReplaceAllString(message, r.replacement)
	}
	return message
}

func (s *LogService) Read(ctx context.Context, request LogRequest, emit func(LogEntry) error) (LogResult, error) {
	if s == nil || s.Docker == nil {
		return LogResult{}, errors.New("log access is unavailable")
	}
	if len(request.Components) < 1 || len(request.Components) > MaxLogComponents {
		return LogResult{}, errors.New("invalid component count")
	}
	limit := request.Limit
	if limit == 0 {
		limit = DefaultLogLines
	}
	if limit < 1 || limit > s.MaxLines {
		return LogResult{}, errors.New("invalid log line limit")
	}
	if request.Follow {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, MaxFollowDuration)
		defer cancel()
	}
	result := LogResult{Entries: make([]LogEntry, 0, min(limit, 256)), Redaction: "best-effort"}
	var bytes int64
	for _, component := range request.Components {
		err := s.Docker.ReadLogs(ctx, component, dockerapi.LogOptions{Since: request.From, Until: request.To, Tail: limit + 1, Follow: request.Follow}, func(stream, raw string) error {
			entry := parseLogEntry(component, stream, s.Redact(raw))
			if request.Search != "" && !strings.Contains(entry.Message, request.Search) {
				return nil
			}
			entryBytes := int64(len(entry.Message) + len(entry.Component) + 64)
			if len(result.Entries) >= limit || bytes+entryBytes > s.MaxBytes {
				result.Truncated = true
				return errLogLimit
			}
			bytes += entryBytes
			if emit != nil {
				return emit(entry)
			}
			result.Entries = append(result.Entries, entry)
			return nil
		})
		if errors.Is(err, errLogLimit) {
			break
		}
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

var errLogLimit = errors.New("log response limit reached")

var jsonLevel = regexp.MustCompile(`(?i)"(?:level|severity)"\s*:\s*"(trace|debug|info|warn(?:ing)?|error|fatal|panic)"`)
var textLevel = regexp.MustCompile(`(?i)(?:^|[\s\[])(trace|debug|info|warn(?:ing)?|error|fatal|panic)(?:[\s\]:]|$)`)

func parseLogEntry(component, stream, raw string) LogEntry {
	entry := LogEntry{Component: component, Stream: stream, Severity: "unknown", Message: raw}
	if first, rest, ok := strings.Cut(raw, " "); ok {
		if at, err := time.Parse(time.RFC3339Nano, first); err == nil {
			entry.Timestamp, entry.Message = at.UTC(), rest
		}
	}
	match := jsonLevel.FindStringSubmatch(entry.Message)
	if len(match) == 0 {
		match = textLevel.FindStringSubmatch(entry.Message)
	}
	if len(match) > 1 {
		entry.Severity = strings.ToLower(match[1])
		if entry.Severity == "warning" {
			entry.Severity = "warn"
		}
	}
	return entry
}
