package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/agent-matrix/matrix-runtime/internal/auth"
)

// UserByEmail returns a user (with workspace) by email, or ErrNotFound. It does
// not reveal whether an email exists to unauthenticated callers — callers in
// the password-reset flow must always respond identically regardless.
func (s *Store) UserByEmail(email string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	u := &User{}
	var created string
	row := s.queryRow(`
		SELECT u.id,u.workspace_id,u.name,u.email,u.role,u.created_at,w.name,w.slug
		FROM users u JOIN workspaces w ON w.id = u.workspace_id WHERE u.email = ?`, email)
	if err := row.Scan(&u.ID, &u.WorkspaceID, &u.Name, &u.Email, &u.Role, &created, &u.WorkspaceName, &u.WorkspaceSlug); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	u.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return u, nil
}

// CreatePasswordReset mints a single-use, time-bounded reset token for a user
// and returns the one-time secret (to be emailed). Only the hash is stored.
func (s *Store) CreatePasswordReset(userID string, ttl time.Duration) (secret string, err error) {
	raw, err := auth.NewToken()
	if err != nil {
		return "", err
	}
	secret = "mxpr_" + raw
	now := time.Now().UTC()
	if ttl <= 0 {
		ttl = time.Hour
	}
	_, err = s.exec(`INSERT INTO password_resets (id,user_id,token_hash,expires_at,used_at,created_at) VALUES (?,?,?,?,'',?)`,
		auth.NewID("pr_"), userID, auth.FastHash(secret), now.Add(ttl).Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		return "", err
	}
	return secret, nil
}

// ResetPassword validates a reset secret and, if valid, sets a new password,
// marks the token used, and revokes all of the user's sessions.
func (s *Store) ResetPassword(secret, newPassword string) error {
	if len(newPassword) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	var id, userID, exp, used string
	row := s.queryRow(`SELECT id,user_id,expires_at,used_at FROM password_resets WHERE token_hash=?`, auth.FastHash(secret))
	if err := row.Scan(&id, &userID, &exp, &used); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("invalid or expired reset link")
		}
		return err
	}
	if used != "" {
		return errors.New("this reset link has already been used")
	}
	if t, e := time.Parse(time.RFC3339, exp); e == nil && time.Now().After(t) {
		return errors.New("this reset link has expired")
	}
	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.exec(`UPDATE users SET password_hash=? WHERE id=?`, hash, userID); err != nil {
		return err
	}
	if _, err := s.exec(`UPDATE password_resets SET used_at=? WHERE id=?`, now, id); err != nil {
		return err
	}
	// Security: invalidate existing sessions after a password change.
	return s.DeleteUserSessions(userID)
}

// CreateEmailVerification mints a single-use email-verification token and
// returns the one-time secret (to be emailed).
func (s *Store) CreateEmailVerification(userID string, ttl time.Duration) (secret string, err error) {
	raw, err := auth.NewToken()
	if err != nil {
		return "", err
	}
	secret = "mxev_" + raw
	now := time.Now().UTC()
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	_, err = s.exec(`INSERT INTO email_verifications (id,user_id,token_hash,expires_at,used_at,created_at) VALUES (?,?,?,?,'',?)`,
		auth.NewID("ev_"), userID, auth.FastHash(secret), now.Add(ttl).Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		return "", err
	}
	return secret, nil
}

// VerifyEmail validates an email-verification secret and marks it used,
// returning the verified user's id.
func (s *Store) VerifyEmail(secret string) (userID string, err error) {
	var id, exp, used string
	row := s.queryRow(`SELECT id,user_id,expires_at,used_at FROM email_verifications WHERE token_hash=?`, auth.FastHash(secret))
	if err := row.Scan(&id, &userID, &exp, &used); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("invalid verification link")
		}
		return "", err
	}
	if used != "" {
		return userID, nil // idempotent: already verified
	}
	if t, e := time.Parse(time.RFC3339, exp); e == nil && time.Now().After(t) {
		return "", errors.New("verification link expired")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.exec(`UPDATE email_verifications SET used_at=? WHERE id=?`, now, id); err != nil {
		return "", err
	}
	return userID, nil
}
