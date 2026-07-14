// SPDX-License-Identifier: AGPL-3.0-only

package notifications

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/drilonrecica/binnacle/internal/alerts"
	"github.com/drilonrecica/binnacle/internal/auth"
)

type Repository struct {
	db               *sql.DB
	secrets          *auth.SecretStore
	reminderInterval time.Duration
}

func NewRepository(db *sql.DB, secrets *auth.SecretStore) *Repository {
	return &Repository{db: db, secrets: secrets, reminderInterval: 2 * time.Hour}
}
func (r *Repository) SetDB(db *sql.DB)                   { r.db = db }
func (r *Repository) SetSecretStore(s *auth.SecretStore) { r.secrets = s }
func (r *Repository) SetReminderInterval(interval time.Duration) {
	if interval > 0 {
		r.reminderInterval = interval
	}
}

func newID(prefix string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func groupFor(a alerts.Alert) (string, string, string) {
	switch {
	case a.Family == alerts.FamilyDockerDown || a.Family == alerts.FamilyPersistence:
		return "subsystem:" + a.TargetID, "subsystem", a.TargetID
	case a.Family == alerts.FamilyHostCPU || a.Family == alerts.FamilyHostMemory:
		return "host:server", "host", "server"
	case a.TargetType == "resource" || a.TargetType == "check":
		return "resource:" + a.TargetID, "resource", a.TargetID
	case a.TargetType == "filesystem":
		return "filesystem:" + a.TargetID, "filesystem", a.TargetID
	case a.TargetType == "host":
		return "host:server", "host", "server"
	default:
		return "resource:" + a.TargetID, a.TargetType, a.TargetID
	}
}

func (r *Repository) AlertFiredTx(ctx context.Context, tx *sql.Tx, a alerts.Alert, now time.Time) error {
	return r.alertFiredTx(ctx, tx, a, now, true)
}

func (r *Repository) alertFiredTx(ctx context.Context, tx *sql.Tx, a alerts.Alert, now time.Time, notify bool) error {
	group, targetType, targetID := groupFor(a)
	var incidentID, severity string
	err := tx.QueryRowContext(ctx, `SELECT id,severity FROM incidents WHERE group_key=? AND status='open'`, group).Scan(&incidentID, &severity)
	opened := errors.Is(err, sql.ErrNoRows)
	if err != nil && !opened {
		return err
	}
	escalated := false
	if opened {
		incidentID = newID("inc_")
		severity = string(a.Severity)
		title := fmt.Sprintf("%s incident on %s", targetType, targetID)
		_, err = tx.ExecContext(ctx, `INSERT INTO incidents(id,group_key,status,severity,target_type,target_id,title,opened_at,updated_at,version,next_reminder_at) VALUES(?,?,?,?,?,?,?,?,?,1,?)`, incidentID, group, "open", severity, targetType, targetID, title, now.Unix(), now.Unix(), now.Add(r.reminderInterval).Unix())
	} else {
		escalated = severity == "warning" && a.Severity == alerts.Critical
		if escalated {
			severity = "critical"
		}
		_, err = tx.ExecContext(ctx, `UPDATE incidents SET severity=?,updated_at=?,version=version+1 WHERE id=?`, severity, now.Unix(), incidentID)
	}
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO incident_alerts(incident_id,alert_id,joined_at) VALUES(?,?,?)`, incidentID, a.ID, now.Unix()); err != nil {
		return err
	}
	if notify {
		event := "updated"
		if opened {
			event = "opened"
		}
		return r.enqueueIncidentTx(ctx, tx, incidentID, event, severity, now.Add(15*time.Second))
	}
	return nil
}

func (r *Repository) AlertResolvedTx(ctx context.Context, tx *sql.Tx, alertID string, now time.Time) error {
	var incidentID string
	if err := tx.QueryRowContext(ctx, `SELECT incident_id FROM incident_alerts WHERE alert_id=?`, alertID).Scan(&incidentID); errors.Is(err, sql.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE incident_alerts SET resolved_at=? WHERE alert_id=? AND resolved_at IS NULL`, now.Unix(), alertID); err != nil {
		return err
	}
	var firing int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM incident_alerts ia JOIN alerts a ON a.id=ia.alert_id WHERE ia.incident_id=? AND a.status='firing'`, incidentID).Scan(&firing); err != nil {
		return err
	}
	if firing > 0 {
		var severity string
		if err := tx.QueryRowContext(ctx, `SELECT CASE WHEN EXISTS(SELECT 1 FROM incident_alerts ia JOIN alerts a ON a.id=ia.alert_id WHERE ia.incident_id=? AND a.status='firing' AND a.severity='critical') THEN 'critical' ELSE 'warning' END`, incidentID).Scan(&severity); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE incidents SET severity=?,updated_at=?,version=version+1 WHERE id=?`, severity, now.Unix(), incidentID)
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE incidents SET status='resolved',resolved_at=?,updated_at=?,next_reminder_at=NULL,version=version+1 WHERE id=?`, now.Unix(), now.Unix(), incidentID); err != nil {
		return err
	}
	return r.enqueueResolutionTx(ctx, tx, incidentID, now)
}

// enqueueResolutionTx only notifies channels where an earlier incident delivery
// was attempted. A still-pending opening is transient noise and is cancelled.
func (r *Repository) enqueueResolutionTx(ctx context.Context, tx *sql.Tx, incidentID string, now time.Time) error {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM notification_channels WHERE enabled=1 AND deleted_at IS NULL AND notify_resolved=1`)
	if err != nil {
		return err
	}
	type target struct{ id string }
	var targets []target
	for rows.Next() {
		var t target
		if err = rows.Scan(&t.id); err != nil {
			rows.Close()
			return err
		}
		targets = append(targets, t)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, ch := range targets {
		var attempted int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_deliveries WHERE incident_id=? AND channel_id=? AND status!='cancelled' AND (attempt_count>0 OR status IN ('in_progress','succeeded','permanent_failure'))`, incidentID, ch.id).Scan(&attempted); err != nil {
			return err
		}
		if attempted == 0 {
			if _, err = tx.ExecContext(ctx, `UPDATE notification_deliveries SET status='cancelled',completed_at=?,next_attempt_at=NULL,updated_at=? WHERE incident_id=? AND channel_id=? AND event_type IN ('opened','updated') AND status='pending' AND attempt_count=0`, now.Unix(), now.Unix(), incidentID, ch.id); err != nil {
				return err
			}
			continue
		}
		// A resolution supersedes queued retries for older lifecycle events. An
		// in-progress attempt is allowed to finish and the worker serializes the
		// resolution behind it.
		if _, err = tx.ExecContext(ctx, `UPDATE notification_deliveries SET status='cancelled',completed_at=?,next_attempt_at=NULL,updated_at=? WHERE incident_id=? AND channel_id=? AND event_type IN ('opened','updated') AND status='pending'`, now.Unix(), now.Unix(), incidentID, ch.id); err != nil {
			return err
		}
		if err = r.upsertDeliveryTx(ctx, tx, incidentID, ch.id, "resolved", now); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) enqueueIncidentTx(ctx context.Context, tx *sql.Tx, incidentID, eventType, severity string, due time.Time) error {
	rows, err := tx.QueryContext(ctx, `SELECT id,minimum_severity,notify_resolved FROM notification_channels WHERE enabled=1 AND deleted_at IS NULL`)
	if err != nil {
		return err
	}
	type target struct {
		id, min  string
		resolved bool
	}
	var targets []target
	for rows.Next() {
		var t target
		if err = rows.Scan(&t.id, &t.min, &t.resolved); err != nil {
			rows.Close()
			return err
		}
		targets = append(targets, t)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, ch := range targets {
		if ch.min == "critical" && severity != "critical" {
			var previouslyNotified int
			if eventType == "resolved" {
				if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_deliveries WHERE incident_id=? AND channel_id=? AND status!='cancelled'`, incidentID, ch.id).Scan(&previouslyNotified); err != nil {
					return err
				}
			}
			if previouslyNotified == 0 {
				continue
			}
		}
		if eventType == "resolved" && !ch.resolved {
			continue
		}
		if err = r.upsertDeliveryTx(ctx, tx, incidentID, ch.id, eventType, due); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) upsertDeliveryTx(ctx context.Context, tx *sql.Tx, incidentID, channelID, eventType string, due time.Time) error {
	var deliveryID, key, existingEvent string
	err := tx.QueryRowContext(ctx, `SELECT id,idempotency_key,event_type FROM notification_deliveries WHERE incident_id=? AND channel_id=? AND status='pending' AND event_type IN ('opened','updated') ORDER BY created_at LIMIT 1`, incidentID, channelID).Scan(&deliveryID, &key, &existingEvent)
	coalesce := eventType == "opened" || eventType == "updated"
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if errors.Is(err, sql.ErrNoRows) || !coalesce {
		deliveryID = newID("del_")
		key = newID("idem_")
		existingEvent = eventType
	}
	payloadEvent := eventType
	if coalesce && err == nil {
		payloadEvent = existingEvent
	}
	payload, err := r.payloadTx(ctx, tx, deliveryID, key, incidentID, payloadEvent)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Unix()
	if coalesce && deliveryID != "" && err == nil {
		var present int
		_ = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_deliveries WHERE id=?`, deliveryID).Scan(&present)
		if present > 0 {
			_, err = tx.ExecContext(ctx, `UPDATE notification_deliveries SET payload_json=?,next_attempt_at=?,updated_at=? WHERE id=?`, payload, due.Unix(), now, deliveryID)
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO notification_deliveries(id,channel_id,incident_id,event_type,payload_json,idempotency_key,status,next_attempt_at,created_at,updated_at) VALUES(?,?,?,?,?,?,'pending',?,?,?)`, deliveryID, channelID, incidentID, eventType, payload, key, due.Unix(), now, now)
	return err
}

func (r *Repository) payloadTx(ctx context.Context, tx *sql.Tx, deliveryID, key, incidentID, eventType string) (string, error) {
	var i Incident
	var opened, updated int64
	var resolved sql.NullInt64
	err := tx.QueryRowContext(ctx, `SELECT id,group_key,status,severity,target_type,target_id,title,opened_at,updated_at,resolved_at,version FROM incidents WHERE id=?`, incidentID).Scan(&i.ID, &i.GroupKey, &i.Status, &i.Severity, &i.TargetType, &i.TargetID, &i.Title, &opened, &updated, &resolved, &i.Version)
	if err != nil {
		return "", err
	}
	i.OpenedAt = time.Unix(opened, 0).UTC()
	i.UpdatedAt = time.Unix(updated, 0).UTC()
	if resolved.Valid {
		v := time.Unix(resolved.Int64, 0).UTC()
		i.ResolvedAt = &v
	}
	rows, err := tx.QueryContext(ctx, `SELECT a.id,a.family,a.severity,a.status,a.message,a.started_at,a.resolved_at FROM incident_alerts ia JOIN alerts a ON a.id=ia.alert_id WHERE ia.incident_id=? ORDER BY a.started_at DESC LIMIT 20`, incidentID)
	if err != nil {
		return "", err
	}
	for rows.Next() {
		var a MemberAlert
		var started int64
		var resolved sql.NullInt64
		if err = rows.Scan(&a.ID, &a.Family, &a.Severity, &a.Status, &a.Message, &started, &resolved); err != nil {
			rows.Close()
			return "", err
		}
		a.StartedAt = time.Unix(started, 0).UTC()
		if resolved.Valid {
			v := time.Unix(resolved.Int64, 0).UTC()
			a.ResolvedAt = &v
		}
		i.Alerts = append(i.Alerts, a)
	}
	rows.Close()
	_ = tx.QueryRowContext(ctx, `SELECT COUNT(*),SUM(CASE WHEN a.status='firing' THEN 1 ELSE 0 END) FROM incident_alerts ia JOIN alerts a ON a.id=ia.alert_id WHERE ia.incident_id=?`, incidentID).Scan(&i.AlertCount, &i.FiringCount)
	v := map[string]any{"schemaVersion": 1, "eventType": eventType, "deliveryId": deliveryID, "idempotencyKey": key, "incident": i}
	b, err := json.Marshal(v)
	return string(b), err
}

func (r *Repository) Reconcile(ctx context.Context, now time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT a.id,a.dedup_key,a.rule_id,a.family,a.severity,a.target_type,a.target_id,a.status,a.started_at,a.last_observed_at,a.observed_value,a.message FROM alerts a LEFT JOIN incident_alerts ia ON ia.alert_id=a.id WHERE a.status='firing' AND ia.alert_id IS NULL ORDER BY a.started_at`)
	if err != nil {
		return err
	}
	var list []alerts.Alert
	for rows.Next() {
		var a alerts.Alert
		var start, last int64
		if err = rows.Scan(&a.ID, &a.DedupKey, &a.RuleID, &a.Family, &a.Severity, &a.TargetType, &a.TargetID, &a.Status, &start, &last, &a.ObservedValue, &a.Message); err != nil {
			rows.Close()
			return err
		}
		a.StartedAt = time.Unix(start, 0).UTC()
		a.LastObservedAt = time.Unix(last, 0).UTC()
		list = append(list, a)
	}
	rows.Close()
	for _, a := range list {
		if err = r.alertFiredTx(ctx, tx, a, now, false); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE incidents SET opened_at=MIN(opened_at,?) WHERE id=(SELECT incident_id FROM incident_alerts WHERE alert_id=?)`, a.StartedAt.Unix(), a.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Repository) Incidents(ctx context.Context, status, severity string, limit, offset int) ([]Incident, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	q := `SELECT i.id,i.group_key,i.status,i.severity,i.target_type,i.target_id,i.title,i.opened_at,i.updated_at,i.resolved_at,i.version,COUNT(ia.alert_id),SUM(CASE WHEN a.status='firing' THEN 1 ELSE 0 END) FROM incidents i LEFT JOIN incident_alerts ia ON ia.incident_id=i.id LEFT JOIN alerts a ON a.id=ia.alert_id WHERE 1=1`
	args := []any{}
	if status != "" {
		q += " AND i.status=?"
		args = append(args, status)
	}
	if severity != "" {
		q += " AND i.severity=?"
		args = append(args, severity)
	}
	q += ` GROUP BY i.id ORDER BY CASE i.status WHEN 'open' THEN 0 ELSE 1 END,CASE i.severity WHEN 'critical' THEN 0 ELSE 1 END,i.opened_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Incident
	for rows.Next() {
		i, err := scanIncident(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

type scanner interface{ Scan(...any) error }

func scanIncident(s scanner) (Incident, error) {
	var i Incident
	var opened, updated int64
	var resolved sql.NullInt64
	err := s.Scan(&i.ID, &i.GroupKey, &i.Status, &i.Severity, &i.TargetType, &i.TargetID, &i.Title, &opened, &updated, &resolved, &i.Version, &i.AlertCount, &i.FiringCount)
	i.OpenedAt = time.Unix(opened, 0).UTC()
	i.UpdatedAt = time.Unix(updated, 0).UTC()
	if resolved.Valid {
		v := time.Unix(resolved.Int64, 0).UTC()
		i.ResolvedAt = &v
	}
	return i, err
}

func (r *Repository) Incident(ctx context.Context, id string) (Incident, error) {
	row := r.db.QueryRowContext(ctx, `SELECT i.id,i.group_key,i.status,i.severity,i.target_type,i.target_id,i.title,i.opened_at,i.updated_at,i.resolved_at,i.version,COUNT(ia.alert_id),COALESCE(SUM(CASE WHEN a.status='firing' THEN 1 ELSE 0 END),0) FROM incidents i LEFT JOIN incident_alerts ia ON ia.incident_id=i.id LEFT JOIN alerts a ON a.id=ia.alert_id WHERE i.id=? GROUP BY i.id`, id)
	value, err := scanIncident(row)
	if err != nil {
		return Incident{}, err
	}
	found := &value
	rows, err := r.db.QueryContext(ctx, `SELECT a.id,a.family,a.severity,a.status,a.message,a.started_at,a.resolved_at FROM incident_alerts ia JOIN alerts a ON a.id=ia.alert_id WHERE ia.incident_id=? ORDER BY a.started_at`, id)
	if err != nil {
		return Incident{}, err
	}
	for rows.Next() {
		var a MemberAlert
		var start int64
		var resolved sql.NullInt64
		if err = rows.Scan(&a.ID, &a.Family, &a.Severity, &a.Status, &a.Message, &start, &resolved); err != nil {
			rows.Close()
			return Incident{}, err
		}
		a.StartedAt = time.Unix(start, 0).UTC()
		if resolved.Valid {
			v := time.Unix(resolved.Int64, 0).UTC()
			a.ResolvedAt = &v
		}
		found.Alerts = append(found.Alerts, a)
	}
	rows.Close()
	found.Deliveries, _ = r.Deliveries(ctx, id, 100, 0)
	return *found, nil
}

func (r *Repository) ExportIncidents(ctx context.Context, from, to time.Time, limit int) ([]Incident, error) {
	if limit < 1 || limit > 10001 {
		limit = 10001
	}
	rows, err := r.db.QueryContext(ctx, `SELECT i.id,i.group_key,i.status,i.severity,i.target_type,i.target_id,i.title,i.opened_at,i.updated_at,i.resolved_at,i.version,COUNT(ia.alert_id),COALESCE(SUM(CASE WHEN a.status='firing' THEN 1 ELSE 0 END),0) FROM incidents i LEFT JOIN incident_alerts ia ON ia.incident_id=i.id LEFT JOIN alerts a ON a.id=ia.alert_id WHERE i.opened_at>=? AND i.opened_at<=? GROUP BY i.id ORDER BY i.opened_at,i.id LIMIT ?`, from.Unix(), to.Unix(), limit)
	if err != nil {
		return nil, err
	}
	values := []Incident{}
	for rows.Next() {
		value, scanErr := scanIncident(rows)
		if scanErr != nil {
			rows.Close()
			return nil, scanErr
		}
		values = append(values, value)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for index := range values {
		value := &values[index]
		alertRows, alertErr := r.db.QueryContext(ctx, `SELECT a.id,a.family,a.severity,a.status,a.message,a.started_at,a.resolved_at FROM incident_alerts ia JOIN alerts a ON a.id=ia.alert_id WHERE ia.incident_id=? ORDER BY a.started_at`, value.ID)
		if alertErr != nil {
			return nil, alertErr
		}
		for alertRows.Next() {
			var alert MemberAlert
			var started int64
			var resolved sql.NullInt64
			if alertErr = alertRows.Scan(&alert.ID, &alert.Family, &alert.Severity, &alert.Status, &alert.Message, &started, &resolved); alertErr != nil {
				alertRows.Close()
				return nil, alertErr
			}
			alert.StartedAt = time.Unix(started, 0).UTC()
			if resolved.Valid {
				at := time.Unix(resolved.Int64, 0).UTC()
				alert.ResolvedAt = &at
			}
			value.Alerts = append(value.Alerts, alert)
		}
		alertRows.Close()
	}
	return values, nil
}

func (r *Repository) Channels(ctx context.Context) ([]Channel, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT c.id,c.name,c.kind,c.enabled,c.minimum_severity,c.notify_resolved,c.config_json,c.secret_ref,c.created_at,c.updated_at,CASE WHEN s.key IS NULL THEN 0 ELSE 1 END FROM notification_channels c LEFT JOIN encrypted_secrets s ON s.key=c.secret_ref WHERE c.deleted_at IS NULL ORDER BY c.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Channel
	for rows.Next() {
		var c Channel
		var config, ref string
		var created, updated int64
		if err = rows.Scan(&c.ID, &c.Name, &c.Kind, &c.Enabled, &c.MinimumSeverity, &c.NotifyResolved, &config, &ref, &created, &updated, &c.SecretConfigured); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(config), &c.Config)
		c.CreatedAt = time.Unix(created, 0).UTC()
		c.UpdatedAt = time.Unix(updated, 0).UTC()
		out = append(out, c)
	}
	return out, rows.Err()
}

func validateChannel(c Channel, s ChannelSecrets) error {
	if strings.TrimSpace(c.Name) == "" || len(c.Name) > 120 {
		return errors.New("channel name is required")
	}
	if c.Kind != "webhook" && c.Kind != "smtp" {
		return errors.New("channel kind must be webhook or smtp")
	}
	if c.MinimumSeverity == "" {
		c.MinimumSeverity = "warning"
	}
	if c.MinimumSeverity != "warning" && c.MinimumSeverity != "critical" {
		return errors.New("invalid minimum severity")
	}
	if c.Kind == "webhook" && s.URL == "" {
		return errors.New("webhook URL is required")
	}
	if c.Kind == "smtp" {
		if s.Host == "" || s.Sender == "" || len(s.Recipients) == 0 || len(s.Recipients) > 20 {
			return errors.New("SMTP host, sender, and 1-20 recipients are required")
		}
		tlsMode, _ := c.Config["tlsMode"].(string)
		if tlsMode != "starttls" && tlsMode != "implicit" {
			return errors.New("SMTP TLS mode must be starttls or implicit")
		}
		if _, err := mail.ParseAddress(s.Sender); err != nil {
			return errors.New("invalid SMTP sender")
		}
		for _, recipient := range s.Recipients {
			if _, err := mail.ParseAddress(recipient); err != nil {
				return errors.New("invalid SMTP recipient")
			}
		}
	}
	return nil
}

func (r *Repository) CreateChannel(ctx context.Context, c Channel, s ChannelSecrets) (Channel, error) {
	var n int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_channels WHERE deleted_at IS NULL`).Scan(&n); err != nil {
		return Channel{}, err
	}
	if n >= 32 {
		return Channel{}, errors.New("notification channel limit reached")
	}
	if c.ID == "" {
		c.ID = newID("chn_")
	}
	if c.MinimumSeverity == "" {
		c.MinimumSeverity = "warning"
	}
	if c.Config == nil {
		c.Config = map[string]any{}
	}
	if err := validateChannel(c, s); err != nil {
		return Channel{}, err
	}
	if r.secrets == nil {
		return Channel{}, auth.ErrMasterKeyMissing
	}
	secretRef := "notification.channel." + c.ID
	b, _ := json.Marshal(s)
	if err := r.secrets.Put(ctx, secretRef, b); err != nil {
		return Channel{}, err
	}
	cfg, _ := json.Marshal(c.Config)
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `INSERT INTO notification_channels(id,name,kind,enabled,minimum_severity,notify_resolved,config_json,secret_ref,created_at,updated_at)VALUES(?,?,?,?,?,?,?,?,?,?)`, c.ID, c.Name, c.Kind, c.Enabled, c.MinimumSeverity, c.NotifyResolved, string(cfg), secretRef, now.Unix(), now.Unix())
	if err != nil {
		_ = r.secrets.Delete(ctx, secretRef)
		return Channel{}, err
	}
	c.SecretConfigured = true
	c.CreatedAt = now
	c.UpdatedAt = now
	return c, nil
}

func (r *Repository) Channel(ctx context.Context, id string) (Channel, error) {
	list, err := r.Channels(ctx)
	if err != nil {
		return Channel{}, err
	}
	for _, c := range list {
		if c.ID == id {
			return c, nil
		}
	}
	return Channel{}, sql.ErrNoRows
}

func (r *Repository) PatchChannel(ctx context.Context, id string, c Channel, patch SecretPatch) (Channel, error) {
	old, err := r.Channel(ctx, id)
	if err != nil {
		return Channel{}, err
	}
	if c.Name == "" {
		c.Name = old.Name
	}
	if c.MinimumSeverity == "" {
		c.MinimumSeverity = old.MinimumSeverity
	}
	if c.Config == nil {
		c.Config = old.Config
	}
	c.ID = id
	c.Kind = old.Kind
	ref := "notification.channel." + id
	var current ChannelSecrets
	b, err := r.secrets.Get(ctx, ref)
	if err != nil {
		return Channel{}, err
	}
	if json.Unmarshal(b, &current) != nil {
		return Channel{}, errors.New("invalid stored channel secret")
	}
	if patch.URL != nil {
		current.URL = *patch.URL
	}
	if patch.BearerToken != nil {
		current.BearerToken = *patch.BearerToken
	}
	if patch.SigningSecret != nil {
		current.SigningSecret = *patch.SigningSecret
	}
	if patch.Host != nil {
		current.Host = *patch.Host
	}
	if patch.Username != nil {
		current.Username = *patch.Username
	}
	if patch.Password != nil {
		current.Password = *patch.Password
	}
	if patch.Sender != nil {
		current.Sender = *patch.Sender
	}
	if patch.Recipients != nil {
		current.Recipients = *patch.Recipients
	}
	if err = validateChannel(c, current); err != nil {
		return Channel{}, err
	}
	b, _ = json.Marshal(current)
	if err = r.secrets.Put(ctx, ref, b); err != nil {
		return Channel{}, err
	}
	cfg, _ := json.Marshal(c.Config)
	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Channel{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE notification_channels SET name=?,enabled=?,minimum_severity=?,notify_resolved=?,config_json=?,updated_at=? WHERE id=? AND deleted_at IS NULL`, c.Name, c.Enabled, c.MinimumSeverity, c.NotifyResolved, string(cfg), now.Unix(), id); err != nil {
		return Channel{}, err
	}
	if !c.Enabled {
		_, err = tx.ExecContext(ctx, `UPDATE notification_deliveries SET status='cancelled',completed_at=?,next_attempt_at=NULL,updated_at=? WHERE channel_id=? AND status='pending'`, now.Unix(), now.Unix(), id)
		if err != nil {
			return Channel{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return Channel{}, err
	}
	return r.Channel(ctx, id)
}
func (r *Repository) PatchedSecrets(ctx context.Context, id string, patch SecretPatch) (ChannelSecrets, error) {
	var current ChannelSecrets
	b, err := r.secrets.Get(ctx, "notification.channel."+id)
	if err != nil {
		return current, err
	}
	if err = json.Unmarshal(b, &current); err != nil {
		return current, errors.New("invalid stored channel secret")
	}
	if patch.URL != nil {
		current.URL = *patch.URL
	}
	if patch.BearerToken != nil {
		current.BearerToken = *patch.BearerToken
	}
	if patch.SigningSecret != nil {
		current.SigningSecret = *patch.SigningSecret
	}
	if patch.Host != nil {
		current.Host = *patch.Host
	}
	if patch.Username != nil {
		current.Username = *patch.Username
	}
	if patch.Password != nil {
		current.Password = *patch.Password
	}
	if patch.Sender != nil {
		current.Sender = *patch.Sender
	}
	if patch.Recipients != nil {
		current.Recipients = *patch.Recipients
	}
	return current, nil
}
func (r *Repository) DeleteChannel(ctx context.Context, id string) error {
	now := time.Now().UTC().Unix()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE notification_channels SET enabled=0,deleted_at=?,updated_at=? WHERE id=? AND deleted_at IS NULL`, now, now, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	if _, err = tx.ExecContext(ctx, `UPDATE notification_deliveries SET status='cancelled',completed_at=?,next_attempt_at=NULL,updated_at=? WHERE channel_id=? AND status='pending'`, now, now, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) Deliveries(ctx context.Context, incident string, limit, offset int) ([]Delivery, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	q := `SELECT id,channel_id,COALESCE(incident_id,''),event_type,idempotency_key,status,attempt_count,next_attempt_at,completed_at,COALESCE(failure_code,''),created_at FROM notification_deliveries WHERE 1=1`
	args := []any{}
	if incident != "" {
		q += ` AND incident_id=?`
		args = append(args, incident)
	}
	q += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Delivery
	for rows.Next() {
		var d Delivery
		var next, completed sql.NullInt64
		var created int64
		if err = rows.Scan(&d.ID, &d.ChannelID, &d.IncidentID, &d.EventType, &d.IdempotencyKey, &d.Status, &d.AttemptCount, &next, &completed, &d.FailureCode, &created); err != nil {
			return nil, err
		}
		d.CreatedAt = time.Unix(created, 0).UTC()
		if next.Valid {
			v := time.Unix(next.Int64, 0).UTC()
			d.NextAttemptAt = &v
		}
		if completed.Valid {
			v := time.Unix(completed.Int64, 0).UTC()
			d.CompletedAt = &v
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (r *Repository) Retry(ctx context.Context, id string) error {
	now := time.Now().Unix()
	res, err := r.db.ExecContext(ctx, `UPDATE notification_deliveries SET status='pending',attempt_count=0,started_at=NULL,next_attempt_at=?,failure_code=NULL,completed_at=NULL,updated_at=? WHERE id=? AND status='permanent_failure'`, now, now, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
func (r *Repository) Test(ctx context.Context, channelID string) (string, error) {
	c, err := r.Channel(ctx, channelID)
	if err != nil {
		return "", err
	}
	id := newID("del_")
	key := newID("idem_")
	payload, _ := json.Marshal(map[string]any{"schemaVersion": 1, "eventType": "test", "deliveryId": id, "idempotencyKey": key})
	now := time.Now().Unix()
	_, err = r.db.ExecContext(ctx, `INSERT INTO notification_deliveries(id,channel_id,event_type,payload_json,idempotency_key,status,next_attempt_at,created_at,updated_at)VALUES(?,?,'test',?,?,'pending',?,?,?)`, id, c.ID, string(payload), key, now, now, now)
	return id, err
}

func (r *Repository) Cleanup(ctx context.Context, now time.Time) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM notification_deliveries WHERE status IN ('succeeded','permanent_failure','cancelled') AND completed_at<?`, now.Add(-90*24*time.Hour).Unix())
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `DELETE FROM incidents WHERE status='resolved' AND resolved_at<?`, now.Add(-365*24*time.Hour).Unix())
	return err
}

func (r *Repository) ScheduleReminders(ctx context.Context, now time.Time, interval time.Duration) error {
	if interval <= 0 {
		interval = 2 * time.Hour
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id,severity FROM incidents WHERE status='open' AND next_reminder_at<=?`, now.Unix())
	if err != nil {
		return err
	}
	type due struct{ id, severity string }
	var list []due
	for rows.Next() {
		var d due
		if err = rows.Scan(&d.id, &d.severity); err != nil {
			rows.Close()
			return err
		}
		list = append(list, d)
	}
	rows.Close()
	for _, d := range list {
		if _, err = tx.ExecContext(ctx, `UPDATE incidents SET next_reminder_at=? WHERE id=?`, now.Add(interval).Unix(), d.id); err != nil {
			return err
		}
		if err = r.enqueueIncidentTx(ctx, tx, d.id, "reminder", d.severity, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Repository) Health(ctx context.Context, dropped int64) (Health, error) {
	var h Health
	h.DroppedDeliveries = dropped
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_deliveries WHERE status='pending'`).Scan(&h.QueueDepth); err != nil {
		return h, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_deliveries WHERE status='permanent_failure'`).Scan(&h.PermanentFailures); err != nil {
		return h, err
	}
	var last sql.NullInt64
	if err := r.db.QueryRowContext(ctx, `SELECT MAX(completed_at) FROM notification_deliveries WHERE status='succeeded'`).Scan(&last); err != nil {
		return h, err
	}
	if last.Valid {
		v := time.Unix(last.Int64, 0).UTC()
		h.LastSuccess = &v
	}
	return h, nil
}
