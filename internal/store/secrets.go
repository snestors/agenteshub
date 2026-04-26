package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Secret is one encrypted entry in the vault.
//
// ValueEnc is AES-GCM ciphertext (cfg.SecretKey). Never returned over HTTP
// — only the metadata fields are exposed in listings; plaintext is fetched
// on-demand via SecretsRepo.GetValue + auth.DecryptAESGCM.
type Secret struct {
	ID             int64
	Key            string
	ValueEnc       []byte
	Description    sql.NullString
	Scope          string // 'global' | 'project:<id>' | 'agent:<name>'
	ExpiresAt      sql.NullInt64
	CreatedAt      int64
	UpdatedAt      int64
	LastAccessedAt sql.NullInt64
}

type SecretsRepo struct{ db *sql.DB }

func NewSecretsRepo(db *sql.DB) *SecretsRepo { return &SecretsRepo{db: db} }

// List returns metadata of every secret. ValueEnc is intentionally omitted
// from the projection so that even a logging slip can't leak ciphertext.
func (r *SecretsRepo) List(ctx context.Context) ([]Secret, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, key, description, scope, expires_at, created_at, updated_at, last_accessed_at
		FROM secrets ORDER BY key ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}
	defer rows.Close()
	out := []Secret{}
	for rows.Next() {
		var s Secret
		if err := rows.Scan(&s.ID, &s.Key, &s.Description, &s.Scope, &s.ExpiresAt,
			&s.CreatedAt, &s.UpdatedAt, &s.LastAccessedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetValue returns the ciphertext for `key`. Caller decrypts with the master.
// Updates last_accessed_at as a side effect.
func (r *SecretsRepo) GetValue(ctx context.Context, key string) ([]byte, error) {
	var enc []byte
	err := r.db.QueryRowContext(ctx, `SELECT value_enc FROM secrets WHERE key = ?`, key).Scan(&enc)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get secret %q: %w", key, err)
	}
	_, _ = r.db.ExecContext(ctx, `UPDATE secrets SET last_accessed_at = ? WHERE key = ?`,
		time.Now().Unix(), key)
	return enc, nil
}

// Upsert creates or rotates a secret. value_enc must already be encrypted.
func (r *SecretsRepo) Upsert(ctx context.Context, s Secret) error {
	now := time.Now().Unix()
	if s.CreatedAt == 0 {
		s.CreatedAt = now
	}
	s.UpdatedAt = now
	if s.Scope == "" {
		s.Scope = "global"
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO secrets(key, value_enc, description, scope, expires_at, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?)
		ON CONFLICT(key) DO UPDATE SET
			value_enc   = excluded.value_enc,
			description = excluded.description,
			scope       = excluded.scope,
			expires_at  = excluded.expires_at,
			updated_at  = excluded.updated_at
	`, s.Key, s.ValueEnc, s.Description, s.Scope, s.ExpiresAt, s.CreatedAt, s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert secret %q: %w", s.Key, err)
	}
	return nil
}

func (r *SecretsRepo) Delete(ctx context.Context, key string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM secrets WHERE key = ?`, key)
	return err
}
