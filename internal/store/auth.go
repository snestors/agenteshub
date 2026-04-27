package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/agenteshub/agenteshub/internal/domain/models"
)

// AuthRepo persists the single user and JWT revocations.
type AuthRepo struct{ db *sql.DB }

// NewAuthRepo creates an AuthRepo.
func NewAuthRepo(db *sql.DB) *AuthRepo { return &AuthRepo{db: db} }

// CreateUser upserts the single AgentHub user.
func (r *AuthRepo) CreateUser(ctx context.Context, u models.User) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO auth_users(id,username,password_hash,totp_secret_encrypted,created_at,last_login) VALUES(1,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET username=excluded.username,password_hash=excluded.password_hash,totp_secret_encrypted=excluded.totp_secret_encrypted`, u.Username, u.PasswordHash, u.TOTPSecretEncrypted, u.CreatedAt, u.LastLogin)
	if err != nil { return fmt.Errorf("create user: %w", err) }
	return nil
}

// GetUserByUsername loads the single user by username.
func (r *AuthRepo) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	return scanUser(r.db.QueryRowContext(ctx, `SELECT id,username,password_hash,totp_secret_encrypted,created_at,last_login FROM auth_users WHERE username=?`, username))
}

// GetUserByID loads the single user by ID.
func (r *AuthRepo) GetUserByID(ctx context.Context, id int64) (*models.User, error) {
	return scanUser(r.db.QueryRowContext(ctx, `SELECT id,username,password_hash,totp_secret_encrypted,created_at,last_login FROM auth_users WHERE id=?`, id))
}

// MarkLogin records the latest login timestamp.
func (r *AuthRepo) MarkLogin(ctx context.Context, id, ts int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE auth_users SET last_login=? WHERE id=?`, ts, id)
	if err != nil { return fmt.Errorf("mark login: %w", err) }
	return nil
}

// RevokeJTI records a JWT ID as revoked until its expiry.
func (r *AuthRepo) RevokeJTI(ctx context.Context, jti string, expiresAt int64) error {
	_, err := r.db.ExecContext(ctx, `INSERT OR REPLACE INTO jwt_revocations(jti,expires_at) VALUES(?,?)`, jti, expiresAt)
	if err != nil { return fmt.Errorf("revoke jti: %w", err) }
	return nil
}

// IsJTIRevoked reports whether a JWT ID is revoked.
func (r *AuthRepo) IsJTIRevoked(ctx context.Context, jti string) (bool, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM jwt_revocations WHERE jti=? AND expires_at > unixepoch()`, jti).Scan(&count); err != nil { return false, fmt.Errorf("check jti revoked: %w", err) }
	return count > 0, nil
}

// DeleteExpiredRevocations removes expired revocation rows.
func (r *AuthRepo) DeleteExpiredRevocations(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM jwt_revocations WHERE expires_at <= unixepoch()`)
	if err != nil { return fmt.Errorf("delete expired revocations: %w", err) }
	return nil
}

func scanUser(row rowScanner) (*models.User, error) {
	var u models.User
	var last sql.NullInt64
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.TOTPSecretEncrypted, &u.CreatedAt, &last); err != nil { return nil, fmt.Errorf("scan user: %w", err) }
	if last.Valid { u.LastLogin = &last.Int64 }
	return &u, nil
}
