// SPDX-License-Identifier: AGPL-3.0-only
package diagnostics

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"
)

func TestBundlePreviewMatchesArchiveAndRedacts(t *testing.T) {
	service := NewBundleService(func(context.Context) BundleData {
		return BundleData{Fields: map[string]any{
			"version": "v0.1.0-alpha.1", "token": "never", "endpoint": "https://user:pass@example.test/path?secret=value",
		}, PartialFailures: []string{"docker version unavailable"}}
	})
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	preview, err := service.Generate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := preview.Fields["token"]; ok {
		t.Fatal("secret field included")
	}
	archive, _, err := service.Download(preview.ID)
	if err != nil {
		t.Fatal(err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		t.Fatal(err)
	}
	reader := tar.NewReader(gz)
	header, err := reader.Next()
	if err != nil || header.Name != "diagnostics.json" {
		t.Fatalf("header=%+v err=%v", header, err)
	}
	payload, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	var archived BundlePreview
	if err = json.Unmarshal(payload, &archived); err != nil || archived.ID != preview.ID || len(archived.PartialFailures) != 1 {
		t.Fatalf("archived=%+v err=%v", archived, err)
	}
	now = now.Add(11 * time.Minute)
	if _, _, err = service.Download(preview.ID); err == nil {
		t.Fatal("expired bundle downloaded")
	}
}
