// SPDX-License-Identifier: AGPL-3.0-only
package diagnostics

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

type BundleData struct {
	Fields          map[string]any `json:"fields"`
	PartialFailures []string       `json:"partialFailures,omitempty"`
}
type BundlePreview struct {
	ID              string         `json:"id"`
	CreatedAt       time.Time      `json:"createdAt"`
	ExpiresAt       time.Time      `json:"expiresAt"`
	Fields          map[string]any `json:"fields"`
	PartialFailures []string       `json:"partialFailures,omitempty"`
}
type bundleEntry struct {
	preview BundlePreview
	archive []byte
}

type BundleService struct {
	mu      sync.Mutex
	entries map[string]bundleEntry
	max     int
	ttl     time.Duration
	now     func() time.Time
	collect func(context.Context) BundleData
}

func NewBundleService(collect func(context.Context) BundleData) *BundleService {
	return &BundleService{entries: map[string]bundleEntry{}, max: 8, ttl: 10 * time.Minute, now: func() time.Time { return time.Now().UTC() }, collect: collect}
}

func (s *BundleService) Generate(ctx context.Context) (BundlePreview, error) {
	if s == nil || s.collect == nil {
		return BundlePreview{}, errors.New("diagnostics collector is unavailable")
	}
	data := s.collect(ctx)
	data.Fields = sanitizeFields(data.Fields)
	now := s.now().UTC()
	id, err := bundleID()
	if err != nil {
		return BundlePreview{}, err
	}
	preview := BundlePreview{ID: id, CreatedAt: now, ExpiresAt: now.Add(s.ttl), Fields: data.Fields, PartialFailures: data.PartialFailures}
	archive, err := bundleArchive(preview)
	if err != nil {
		return BundlePreview{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, entry := range s.entries {
		if !now.Before(entry.preview.ExpiresAt) {
			delete(s.entries, key)
		}
	}
	if len(s.entries) >= s.max {
		oldest := ""
		for key, entry := range s.entries {
			if oldest == "" || entry.preview.CreatedAt.Before(s.entries[oldest].preview.CreatedAt) {
				oldest = key
			}
		}
		delete(s.entries, oldest)
	}
	s.entries[id] = bundleEntry{preview: preview, archive: archive}
	return preview, nil
}

func (s *BundleService) Download(id string) ([]byte, BundlePreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[id]
	if !ok || !s.now().Before(entry.preview.ExpiresAt) {
		delete(s.entries, id)
		return nil, BundlePreview{}, errors.New("diagnostics preview expired or was not found")
	}
	return append([]byte(nil), entry.archive...), entry.preview, nil
}

func sanitizeFields(fields map[string]any) map[string]any {
	result := map[string]any{}
	for key, value := range fields {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "password") || strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "environment") || strings.Contains(lower, "logs") {
			continue
		}
		switch typed := value.(type) {
		case string:
			result[key] = sanitizeDetail(typed)
		case map[string]any:
			result[key] = sanitizeFields(typed)
		default:
			result[key] = value
		}
	}
	return result
}

func bundleArchive(preview BundlePreview) ([]byte, error) {
	payload, err := json.MarshalIndent(preview, "", "  ")
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	gz := gzip.NewWriter(&output)
	tarWriter := tar.NewWriter(gz)
	header := &tar.Header{Name: "diagnostics.json", Mode: 0o600, Size: int64(len(payload)), ModTime: preview.CreatedAt}
	if err = tarWriter.WriteHeader(header); err == nil {
		_, err = tarWriter.Write(payload)
	}
	if closeErr := tarWriter.Close(); err == nil {
		err = closeErr
	}
	if closeErr := gz.Close(); err == nil {
		err = closeErr
	}
	return output.Bytes(), err
}

func bundleID() (string, error) {
	value := make([]byte, 18)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return "diag_" + base64.RawURLEncoding.EncodeToString(value), nil
}

func SortedFieldNames(fields map[string]any) []string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
