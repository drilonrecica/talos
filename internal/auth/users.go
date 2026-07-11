// SPDX-License-Identifier: AGPL-3.0-only
package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

var (
	ErrAdminExists        = errors.New("local admin already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

type User struct {
	ID        string
	Username  string
	CreatedAt time.Time
}

type Credentials struct{ db *sql.DB }

func NewCredentials(db *sql.DB) *Credentials { return &Credentials{db: db} }
func (c *Credentials) SetDB(db *sql.DB)      { c.db = db }

func (c *Credentials) CreateAdmin(ctx context.Context, username, password string) (User, error) {
	if c == nil || c.db == nil {
		return User{}, errors.New("credential repository is unavailable")
	}
	username, err := ValidateUsername(username)
	if err != nil {
		return User{}, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()
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
	now := time.Now().UTC()
	if _, err = tx.ExecContext(ctx, "INSERT INTO users(id,username,password_hash,created_at,updated_at) VALUES(?,?,?,?,?)", id, username, hash, now.UnixMilli(), now.UnixMilli()); err != nil {
		if isUnique(err) {
			return User{}, ErrAdminExists
		}
		return User{}, err
	}
	if err = tx.Commit(); err != nil {
		return User{}, err
	}
	return User{ID: id, Username: username, CreatedAt: now}, nil
}

func (c *Credentials) Authenticate(ctx context.Context, username, password string) (User, error) {
	if c == nil || c.db == nil {
		return User{}, ErrInvalidCredentials
	}
	username, err := ValidateUsername(username)
	if err != nil {
		_ = VerifyPassword(dummyPasswordHash, password)
		return User{}, ErrInvalidCredentials
	}
	var u User
	var created int64
	var hash string
	err = c.db.QueryRowContext(ctx, "SELECT id,username,password_hash,created_at FROM users WHERE username=?", username).Scan(&u.ID, &u.Username, &hash, &created)
	if err != nil {
		// Keep the missing-user path comparably expensive.
		_ = VerifyPassword(dummyPasswordHash, password)
		return User{}, ErrInvalidCredentials
	}
	if !VerifyPassword(hash, password) {
		return User{}, ErrInvalidCredentials
	}
	u.CreatedAt = time.UnixMilli(created).UTC()
	return u, nil
}

func (c *Credentials) UserByID(ctx context.Context, id string) (User, error) {
	if c == nil || c.db == nil || id == "" {
		return User{}, ErrInvalidCredentials
	}
	var user User
	var created int64
	if err := c.db.QueryRowContext(ctx, "SELECT id,username,created_at FROM users WHERE id=?", id).Scan(&user.ID, &user.Username, &created); err != nil {
		return User{}, ErrInvalidCredentials
	}
	user.CreatedAt = time.UnixMilli(created).UTC()
	return user, nil
}

func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random id: %w", err)
	}
	return "usr_" + hex.EncodeToString(b), nil
}

// This valid hash is only used to equalize invalid-username and absent-user work.
const dummyPasswordHash = "$argon2id$v=19$m=65536,t=3,p=4$MDEyMzQ1Njc4OWFiY2RlZg$Dncpc0A4KDsA4DCI5PJq5HR1uGPs2DG8hu6ZUHnLK14"

func isUnique(err error) bool {
	return err != nil && (contains(err.Error(), "UNIQUE") || contains(err.Error(), "unique"))
}
func contains(s, want string) bool {
	for i := 0; i+len(want) <= len(s); i++ {
		if s[i:i+len(want)] == want {
			return true
		}
	}
	return false
}
