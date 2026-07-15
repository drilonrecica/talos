// SPDX-License-Identifier: AGPL-3.0-only
package diagnostics

import (
	"context"
	"strings"
	"testing"

	"github.com/drilonrecica/binnacle/internal/dockerapi"
)

type fakeLogs struct{ lines []string }

func (f fakeLogs) ReadLogs(ctx context.Context, _ string, _ dockerapi.LogOptions, emit func(string, string) error) error {
	for _, line := range f.lines {
		if err := emit("stdout", line); err != nil {
			return err
		}
	}
	return nil
}

func TestLogServiceRedactsCompleteSecretValues(t *testing.T) {
	service, err := NewLogService(fakeLogs{}, 5000, 1<<20, []string{`session-[a-z]+`})
	if err != nil {
		t.Fatal(err)
	}
	privateKey := "-----BEGIN PRIVATE KEY-----\nprivate material\n-----END PRIVATE KEY-----"
	tests := map[string]string{
		`{"password":"correct horse battery staple"}`:         `{"password":"[REDACTED]"}`,
		`secret = 'two words with \'escaped\' quotes'`:        `secret = '[REDACTED]'`,
		`{"api_key": "value with \"escaped\" quotes"}`:        `{"api_key": "[REDACTED]"}`,
		`password=correct horse battery staple`:               `password=[REDACTED]`,
		`token=abc123; user=admin`:                            `token=[REDACTED]; user=admin`,
		`Authorization: Basic dXNlcjpwYXNz`:                   `Authorization: Basic [REDACTED]`,
		`postgres://user:correct-horse@example.test/database`: `postgres://user:[REDACTED]@example.test/database`,
		`request session-secret completed`:                    `request [REDACTED] completed`,
		privateKey:                                            `[REDACTED PRIVATE KEY]`,
	}
	for input, want := range tests {
		if got := service.Redact(input); got != want {
			t.Errorf("Redact(%q)=%q, want %q", input, got, want)
		}
		if strings.Contains(service.Redact(input), "correct horse") {
			t.Errorf("Redact(%q) retained secret text", input)
		}
	}
}

func TestLogServiceRedactsBeforeLiteralSearchAndPreservesMessage(t *testing.T) {
	service, err := NewLogService(fakeLogs{[]string{
		`2026-01-02T03:04:05Z {"level":"error","token":"secret"}`,
		`Authorization: Bearer abc123`,
	}}, 5000, 1<<20, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Read(context.Background(), LogRequest{Components: []string{"container"}, Limit: 10, Search: "[REDACTED]"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("entries=%d", len(result.Entries))
	}
	if result.Entries[0].Severity != "error" {
		t.Fatalf("severity=%q", result.Entries[0].Severity)
	}
	if result.Entries[0].Message != `{"level":"error","token":"[REDACTED]"}` {
		t.Fatalf("message=%q", result.Entries[0].Message)
	}
}

func TestLogServiceReportsTruncation(t *testing.T) {
	service, _ := NewLogService(fakeLogs{[]string{"one", "two", "three"}}, 5000, 1<<20, nil)
	result, err := service.Read(context.Background(), LogRequest{Components: []string{"container"}, Limit: 2}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Truncated || len(result.Entries) != 2 {
		t.Fatalf("result=%+v", result)
	}
}
