// SPDX-License-Identifier: AGPL-3.0-only
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

var (
	ErrSetupUnavailable = errors.New("setup is unavailable")
	ErrSetupToken       = errors.New("setup token is invalid")
)

type SetupService struct {
	db       *sql.DB
	now      func() time.Time
	tokenTTL time.Duration
}

func NewSetupService(db *sql.DB) *SetupService {
	return &SetupService{db: db, now: func() time.Time { return time.Now().UTC() }, tokenTTL: 24 * time.Hour}
}

// SetupTokenFromEnvironment reads the operator-provided setup token. A file
// source is intended for Docker secrets and takes precedence only when the
// direct value is absent; configuring both is rejected to avoid ambiguity.
func SetupTokenFromEnvironment() (string, error) {
	return EnvironmentSecret("BINNACLE_SETUP_TOKEN")
}
func (s *SetupService) SetDB(db *sql.DB) { s.db = db }

// Disable permanently records that first-run setup has completed. It is used
// by non-browser bootstrap paths as well as the browser claim transaction.
func (s *SetupService) Disable(ctx context.Context) error {
	if s == nil || s.db == nil {
		return ErrSetupUnavailable
	}
	now := s.now().UTC().UnixMilli()
	_, err := s.db.ExecContext(ctx, `INSERT INTO setup_state(id,token_hash,expires_at,claimed_at,created_at)
		VALUES(1,NULL,NULL,?,?) ON CONFLICT(id) DO UPDATE SET token_hash=NULL,claimed_at=COALESCE(setup_state.claimed_at,excluded.claimed_at)`, now, now)
	return err
}

// Initialize creates an operator token only for loopback-only installations. A
// public listener must be supplied an operator token out of band.
func (s *SetupService) Initialize(ctx context.Context, listenAddress, configuredToken string) (generated string, err error) {
	if s == nil || s.db == nil {
		return "", ErrSetupUnavailable
	}
	var users int
	if err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&users); err != nil {
		return "", err
	}
	if users > 0 {
		return "", nil
	}
	var claimed sql.NullInt64
	err = s.db.QueryRowContext(ctx, "SELECT claimed_at FROM setup_state WHERE id=1").Scan(&claimed)
	if err == nil {
		if claimed.Valid {
			return "", nil
		}
		return "", nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	public := publicListener(listenAddress)
	token := strings.TrimSpace(configuredToken)
	if token == "" {
		if public {
			return "", fmt.Errorf("public setup requires BINNACLE_SETUP_TOKEN")
		}
		var e error
		token, e = randomSetupToken()
		if e != nil {
			return "", e
		}
		generated = token
	}
	if len(token) < 32 {
		return "", fmt.Errorf("setup token must be at least 32 characters")
	}
	now := s.now().UTC()
	_, err = s.db.ExecContext(ctx, "INSERT INTO setup_state(id,token_hash,expires_at,created_at) VALUES(1,?,?,?)", setupHash(token), now.Add(s.tokenTTL).UnixMilli(), now.UnixMilli())
	return generated, err
}

func (s *SetupService) Available(ctx context.Context) bool {
	if s == nil || s.db == nil {
		return false
	}
	var token string
	var expires int64
	var claimed sql.NullInt64
	if err := s.db.QueryRowContext(ctx, "SELECT COALESCE(token_hash,''),COALESCE(expires_at,0),claimed_at FROM setup_state WHERE id=1").Scan(&token, &expires, &claimed); err != nil {
		return false
	}
	return token != "" && !claimed.Valid && s.now().UTC().UnixMilli() < expires
}

func (s *SetupService) Verify(ctx context.Context, token string) error {
	if s == nil || s.db == nil {
		return ErrSetupUnavailable
	}
	var expected string
	var expires int64
	var claimed sql.NullInt64
	if err := s.db.QueryRowContext(ctx, "SELECT COALESCE(token_hash,''),COALESCE(expires_at,0),claimed_at FROM setup_state WHERE id=1").Scan(&expected, &expires, &claimed); err != nil {
		return ErrSetupUnavailable
	}
	if expected == "" || claimed.Valid || s.now().UTC().UnixMilli() >= expires || subtle.ConstantTimeCompare([]byte(expected), []byte(setupHash(token))) != 1 {
		return ErrSetupToken
	}
	return nil
}

func (s *SetupService) Claim(ctx context.Context, token, username, password string) (User, error) {
	username, err := ValidateUsername(username)
	if err != nil {
		return User{}, err
	}
	if err = ValidatePassword(password); err != nil {
		return User{}, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()
	var expected string
	var expires int64
	var claimed sql.NullInt64
	if err = tx.QueryRowContext(ctx, "SELECT COALESCE(token_hash,''),COALESCE(expires_at,0),claimed_at FROM setup_state WHERE id=1").Scan(&expected, &expires, &claimed); err != nil {
		return User{}, ErrSetupUnavailable
	}
	now := s.now().UTC()
	if expected == "" || claimed.Valid || now.UnixMilli() >= expires || subtle.ConstantTimeCompare([]byte(expected), []byte(setupHash(token))) != 1 {
		return User{}, ErrSetupToken
	}
	var count int
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return User{}, err
	}
	if count != 0 {
		return User{}, ErrAdminExists
	}
	id, err := randomID()
	if err != nil {
		return User{}, err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO users(id,username,password_hash,created_at,updated_at) VALUES(?,?,?,?,?)", id, username, hash, now.UnixMilli(), now.UnixMilli()); err != nil {
		return User{}, err
	}
	r, err := tx.ExecContext(ctx, "UPDATE setup_state SET claimed_at=?,token_hash=NULL WHERE id=1 AND claimed_at IS NULL", now.UnixMilli())
	if err != nil {
		return User{}, err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return User{}, ErrSetupUnavailable
	}
	if err = tx.Commit(); err != nil {
		return User{}, err
	}
	return User{ID: id, Username: username, CreatedAt: now}, nil
}

func publicListener(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return true
	}
	host = strings.Trim(host, "[]")
	if host == "localhost" {
		return false
	}
	ip := net.ParseIP(host)
	return ip == nil || !ip.IsLoopback()
}
func randomSetupToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b), err
}
func setupHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawStdEncoding.EncodeToString(sum[:])
}
