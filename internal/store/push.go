package store

import (
	"context"
	"database/sql"
)

type PushToken struct {
	ID       int64
	Provider string
	Token    string
}

type PushRepo struct{ db *sql.DB }

func NewPushRepo(db *sql.DB) *PushRepo { return &PushRepo{db: db} }

func (r *PushRepo) Upsert(ctx context.Context, provider, token, userAgent string) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO push_tokens(provider, token, user_agent, created_at, updated_at)
        VALUES(?, ?, ?, unixepoch(), unixepoch())
        ON CONFLICT(token) DO UPDATE SET
          provider=excluded.provider,
          user_agent=excluded.user_agent,
          updated_at=unixepoch(),
          disabled_at=NULL,
          last_error=NULL
    `, provider, token, userAgent)
	return err
}

func (r *PushRepo) Active(ctx context.Context, provider string) ([]PushToken, error) {
	rows, err := r.db.QueryContext(ctx, `
        SELECT id, provider, token
        FROM push_tokens
        WHERE provider = ? AND disabled_at IS NULL
        ORDER BY updated_at DESC
    `, provider)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PushToken{}
	for rows.Next() {
		var t PushToken
		if err := rows.Scan(&t.ID, &t.Provider, &t.Token); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *PushRepo) MarkError(ctx context.Context, token, errMsg string, disable bool) error {
	if disable {
		_, err := r.db.ExecContext(ctx, `
            UPDATE push_tokens SET last_error = ?, disabled_at = unixepoch(), updated_at = unixepoch()
            WHERE token = ?
        `, errMsg, token)
		return err
	}
	_, err := r.db.ExecContext(ctx, `
        UPDATE push_tokens SET last_error = ?, updated_at = unixepoch()
        WHERE token = ?
    `, errMsg, token)
	return err
}
