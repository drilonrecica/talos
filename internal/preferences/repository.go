// SPDX-License-Identifier: AGPL-3.0-only
package preferences

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const SchemaVersion = 1

var ErrInvalid = errors.New("preferences are invalid")

type Value struct {
	SchemaVersion   int       `json:"schemaVersion"`
	Theme           string    `json:"theme"`
	Density         string    `json:"density"`
	PinnedResources []string  `json:"pinnedResources"`
	LandingPage     string    `json:"landingPage"`
	ChartRange      string    `json:"chartRange"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type Repository struct {
	db  *sql.DB
	now func() time.Time
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, now: func() time.Time { return time.Now().UTC() }}
}
func (r *Repository) SetDB(db *sql.DB) { r.db = db }

func (r *Repository) Get(ctx context.Context, userID string) (Value, bool, error) {
	var value Value
	var pins string
	var updated int64
	err := r.db.QueryRowContext(ctx, `SELECT schema_version,theme,density,pinned_resources_json,landing_page,chart_range,updated_at FROM user_preferences WHERE user_id=?`, userID).
		Scan(&value.SchemaVersion, &value.Theme, &value.Density, &pins, &value.LandingPage, &value.ChartRange, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return Value{}, false, nil
	}
	if err != nil {
		return Value{}, false, err
	}
	if json.Unmarshal([]byte(pins), &value.PinnedResources) != nil || validate(value) != nil {
		return Value{}, false, errors.New("stored preferences are invalid")
	}
	value.UpdatedAt = time.UnixMilli(updated).UTC()
	return value, true, nil
}

func (r *Repository) Put(ctx context.Context, userID string, value Value) (Value, error) {
	value.SchemaVersion = SchemaVersion
	value.PinnedResources = append([]string(nil), value.PinnedResources...)
	if err := validate(value); err != nil {
		return Value{}, err
	}
	pins, err := json.Marshal(value.PinnedResources)
	if err != nil {
		return Value{}, ErrInvalid
	}
	value.UpdatedAt = r.now()
	_, err = r.db.ExecContext(ctx, `INSERT INTO user_preferences(user_id,schema_version,theme,density,pinned_resources_json,landing_page,chart_range,updated_at)
		VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(user_id) DO UPDATE SET schema_version=excluded.schema_version,theme=excluded.theme,density=excluded.density,pinned_resources_json=excluded.pinned_resources_json,landing_page=excluded.landing_page,chart_range=excluded.chart_range,updated_at=excluded.updated_at`,
		userID, value.SchemaVersion, value.Theme, value.Density, string(pins), value.LandingPage, value.ChartRange, value.UpdatedAt.UnixMilli())
	if err != nil {
		return Value{}, err
	}
	return value, nil
}

func validate(value Value) error {
	if value.SchemaVersion != SchemaVersion || !oneOf(value.Theme, "system", "dark", "light") || !oneOf(value.Density, "comfortable", "compact") || !oneOf(value.LandingPage, "watch", "resources", "server", "events", "alerts") || !oneOf(value.ChartRange, "1h", "6h", "24h", "7d", "30d") || len(value.PinnedResources) > 12 {
		return ErrInvalid
	}
	seen := make(map[string]bool, len(value.PinnedResources))
	for _, id := range value.PinnedResources {
		if id != strings.TrimSpace(id) || len(id) < 1 || len(id) > 128 || seen[id] {
			return ErrInvalid
		}
		seen[id] = true
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
