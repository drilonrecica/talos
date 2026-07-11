// SPDX-License-Identifier: AGPL-3.0-only
package settings

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"
)

var ErrRevisionConflict = errors.New("settings revision conflict")

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store  { return &Store{db: db} }
func (s *Store) SetDB(db *sql.DB) { s.db = db }

func (s *Store) Overrides() (map[string]string, error) {
	return s.overrides(context.Background(), s.db)
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func (s *Store) overrides(ctx context.Context, db queryer) (map[string]string, error) {
	if db == nil {
		return nil, errors.New("settings storage is unavailable")
	}
	rows, err := db.QueryContext(ctx, "SELECT key,value_json FROM settings ORDER BY key")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := map[string]string{}
	for rows.Next() {
		var key, raw, value string
		if err = rows.Scan(&key, &raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(raw), &value); err != nil {
			return nil, fmt.Errorf("decode setting %s: %w", key, err)
		}
		values[key] = value
	}
	return values, rows.Err()
}

type ApplyMode string

const (
	ApplyLive    ApplyMode = "live"
	ApplyRestart ApplyMode = "restart_required"
)

type SettingView struct {
	Value     string    `json:"value"`
	Source    Source    `json:"source"`
	ApplyMode ApplyMode `json:"applyMode"`
}
type Snapshot struct {
	Revision int64                  `json:"revision"`
	Values   map[string]SettingView `json:"values"`
}

type Service struct {
	mu        sync.RWMutex
	store     *Store
	base      Config
	effective map[string]Effective
	current   Config
	apply     func(Config)
}

func NewService(store *Store, base Config, effective map[string]Effective, apply func(Config)) *Service {
	return &Service{store: store, base: base, effective: effective, current: base, apply: apply}
}
func (s *Service) SetDB(db *sql.DB) { s.store.SetDB(db) }
func (s *Service) Initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	overrides, err := s.store.overrides(ctx, s.store.db)
	if err != nil {
		return err
	}
	resolved, err := ResolveOverrides(s.base, overrides)
	if err != nil {
		return err
	}
	s.current = resolved
	if s.apply != nil {
		s.apply(resolved)
	}
	return nil
}

var editable = map[string]ApplyMode{
	"collection.host_interval": ApplyLive, "collection.container_interval": ApplyLive,
	"persistence.raw_interval": ApplyLive, "retention.preset": ApplyLive,
	"retention.raw": ApplyLive, "retention.one_minute": ApplyLive,
	"retention.fifteen_minute": ApplyLive, "retention.one_hour": ApplyLive,
	"database.target_budget_bytes": ApplyLive, "charts.max_points_per_series": ApplyLive,
	"sessions.idle_timeout": ApplyLive, "sessions.absolute_lifetime": ApplyLive,
}
var visible = func() map[string]ApplyMode {
	result := map[string]ApplyMode{
		"paths.data_dir": ApplyRestart, "http.listen_address": ApplyRestart,
		"docker.socket_path": ApplyRestart, "paths.host_proc": ApplyRestart, "paths.host_sys": ApplyRestart,
	}
	for key, mode := range editable {
		result[key] = mode
	}
	return result
}()

func (s *Service) Snapshot(ctx context.Context) (Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	overrides, err := s.store.overrides(ctx, s.store.db)
	if err != nil {
		return Snapshot{}, err
	}
	revision, err := revision(ctx, s.store.db)
	if err != nil {
		return Snapshot{}, err
	}
	resolved, err := ResolveOverrides(s.base, overrides)
	if err != nil {
		return Snapshot{}, err
	}
	values := map[string]SettingView{}
	for key, mode := range visible {
		source := SourceDefault
		if base, ok := s.effective[key]; ok {
			source = base.Source
		}
		if _, ok := overrides[key]; ok {
			source = SourceAdmin
		}
		values[key] = SettingView{Value: lookup(resolved, key), Source: source, ApplyMode: mode}
	}
	return Snapshot{Revision: revision, Values: values}, nil
}

func (s *Service) Patch(ctx context.Context, expected int64, changes map[string]string, actor string) (Snapshot, error) {
	if len(changes) == 0 || len(changes) > 16 {
		return Snapshot{}, errors.New("one to sixteen settings changes are required")
	}
	for key := range changes {
		if _, ok := editable[key]; !ok || !UIOverridable(key) {
			return Snapshot{}, fmt.Errorf("setting %s is not editable", key)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return Snapshot{}, err
	}
	defer tx.Rollback()
	currentRevision, err := revision(ctx, tx)
	if err != nil {
		return Snapshot{}, err
	}
	if currentRevision != expected {
		return Snapshot{}, ErrRevisionConflict
	}
	current, err := s.store.overrides(ctx, tx)
	if err != nil {
		return Snapshot{}, err
	}
	merged := make(map[string]string, len(current)+len(changes))
	for key, value := range current {
		merged[key] = value
	}
	for key, value := range changes {
		merged[key] = value
	}
	resolved, err := ResolveOverrides(s.base, merged)
	if err != nil {
		return Snapshot{}, err
	}
	next := currentRevision + 1
	now := time.Now().UTC().UnixMilli()
	for key, value := range changes {
		raw, _ := json.Marshal(value)
		var previous any
		if old, ok := current[key]; ok {
			encoded, _ := json.Marshal(old)
			previous = string(encoded)
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO settings(key,value_json,updated_at,updated_by) VALUES(?,?,?,?)
			ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json,updated_at=excluded.updated_at,updated_by=excluded.updated_by`, key, string(raw), now, actor); err != nil {
			return Snapshot{}, err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO settings_audit(revision,setting_key,previous_value_json,new_value_json,actor,changed_at) VALUES(?,?,?,?,?,?)", next, key, previous, string(raw), actor, now); err != nil {
			return Snapshot{}, err
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE application_metadata SET value=? WHERE key='settings_revision'", strconv.FormatInt(next, 10)); err != nil {
		return Snapshot{}, err
	}
	if err = tx.Commit(); err != nil {
		return Snapshot{}, err
	}
	s.current = resolved
	if s.apply != nil {
		s.apply(resolved)
	}
	return s.snapshotLocked(ctx)
}

func (s *Service) snapshotLocked(ctx context.Context) (Snapshot, error) {
	overrides, err := s.store.overrides(ctx, s.store.db)
	if err != nil {
		return Snapshot{}, err
	}
	rev, err := revision(ctx, s.store.db)
	if err != nil {
		return Snapshot{}, err
	}
	values := map[string]SettingView{}
	for key, mode := range visible {
		source := SourceDefault
		if base, ok := s.effective[key]; ok {
			source = base.Source
		}
		if _, ok := overrides[key]; ok {
			source = SourceAdmin
		}
		values[key] = SettingView{Value: lookup(s.current, key), Source: source, ApplyMode: mode}
	}
	return Snapshot{Revision: rev, Values: values}, nil
}

type rower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func revision(ctx context.Context, db rower) (int64, error) {
	var raw string
	if err := db.QueryRowContext(ctx, "SELECT value FROM application_metadata WHERE key='settings_revision'").Scan(&raw); err != nil {
		return 0, err
	}
	return strconv.ParseInt(raw, 10, 64)
}
