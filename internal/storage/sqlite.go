// SPDX-License-Identifier: AGPL-3.0-only

package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/drilonrecica/binnacle/migrations"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/sys/unix"
)

const MinimumFreeSpace = 64 << 20

// Manager owns Binnacle's SQLite connection and acts as an app lifecycle component.
type Manager struct {
	Path, RuntimeDir string
	db               *sql.DB
	mu               sync.Mutex
	workerCancel     context.CancelFunc
	workerWG         sync.WaitGroup
	prevCollectors   map[string]string
	retention        RetentionCutoffs
	budget           DatabaseBudget
	emergencyPause   bool
}

// RetentionCutoffs configures how long each resolution tier is kept.
type RetentionCutoffs struct {
	Raw, OneMinute, FifteenMinute, OneHour time.Duration
}

// DatabaseBudget exposes the current budget state to callers.
type DatabaseBudget struct {
	UsedBytes   int64
	TargetBytes int64
	State       BudgetState
}

func New(path, runtimeDir string) *Manager { return &Manager{Path: path, RuntimeDir: runtimeDir} }
func (m *Manager) Start(ctx context.Context) error {
	if err := m.Open(ctx); err != nil {
		return err
	}
	workerCtx, cancel := context.WithCancel(ctx)
	if err := m.recoverDeletionJobs(ctx); err != nil {
		cancel()
		_ = m.Close()
		return err
	}
	m.mu.Lock()
	m.workerCancel = cancel
	m.mu.Unlock()
	m.workerWG.Add(3)
	go func() { defer m.workerWG.Done(); m.runDeletionWorker(workerCtx) }()
	go func() { defer m.workerWG.Done(); m.runRollups(workerCtx) }()
	go func() { defer m.workerWG.Done(); m.runRetentionAndBudget(workerCtx) }()
	return nil
}
func (m *Manager) Stop(context.Context) error {
	m.mu.Lock()
	cancel := m.workerCancel
	m.workerCancel = nil
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	m.workerWG.Wait()
	return m.Close()
}
func (m *Manager) DB() *sql.DB { return m.db }
func (m *Manager) SchemaVersion(ctx context.Context) (int, error) {
	if m.db == nil {
		return 0, errors.New("storage is not open")
	}
	var v int
	err := m.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&v)
	return v, err
}
func (m *Manager) Open(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.db != nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(m.Path), 0750); err != nil {
		return err
	}
	if err := os.MkdirAll(m.RuntimeDir, 0750); err != nil {
		return err
	}
	marker := m.markerPath()
	if _, err := os.Stat(marker); err == nil {
		return fmt.Errorf("migration failure marker exists at %s; resolve it before retrying", marker)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := freeSpace(filepath.Dir(m.Path)); err != nil {
		return err
	}
	db, err := sql.Open("sqlite3", m.Path)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	fail := func(err error) error { _ = db.Close(); return err }
	for _, pragma := range []string{"PRAGMA journal_mode=WAL", "PRAGMA foreign_keys=ON", "PRAGMA busy_timeout=5000", "PRAGMA synchronous=NORMAL"} {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			return fail(fmt.Errorf("sqlite pragma: %w", err))
		}
	}
	var integrity string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil {
		return fail(err)
	}
	if integrity != "ok" {
		return fail(fmt.Errorf("sqlite integrity check failed: %s", sanitize(integrity)))
	}
	if err := m.migrate(ctx, db); err != nil {
		_ = writeMarker(marker)
		return fail(err)
	}
	m.db = db
	return nil
}
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.db == nil {
		return nil
	}
	_, _ = m.db.Exec("PRAGMA wal_checkpoint(PASSIVE)")
	err := m.db.Close()
	m.db = nil
	return err
}
func (m *Manager) migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)"); err != nil {
		return err
	}
	entries, err := fs.Glob(migrations.FS(), "*.sql")
	if err != nil {
		return err
	}
	sort.Strings(entries)
	for _, entry := range entries {
		base := filepath.Base(entry)
		var version int
		if _, err := fmt.Sscanf(base, "%03d_", &version); err != nil {
			return fmt.Errorf("invalid migration name %s", base)
		}
		var present int
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version=?", version).Scan(&present)
		if err != nil {
			return err
		}
		if present > 0 {
			continue
		}
		sqlBytes, err := migrations.FS().ReadFile(entry)
		if err != nil {
			return err
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, string(sqlBytes)); err == nil {
			_, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations(version, applied_at) VALUES(?, datetime('now'))", version)
		}
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %03d failed: %w", version, err)
		}
		if err = tx.Commit(); err != nil {
			return fmt.Errorf("migration %03d commit: %w", version, err)
		}
	}
	return nil
}
func (m *Manager) markerPath() string { return filepath.Join(m.RuntimeDir, "migration.failed") }
func freeSpace(path string) error {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return err
	}
	if stat.Bavail*uint64(stat.Bsize) < MinimumFreeSpace {
		return fmt.Errorf("insufficient free disk space for migration (need %d bytes)", MinimumFreeSpace)
	}
	return nil
}
func writeMarker(path string) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte("migration failed; inspect application logs and restore or repair the database before removing this marker\n"), 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
func sanitize(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 200 {
		return s[:200]
	}
	return s
}
