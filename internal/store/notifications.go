package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Notification is the persisted record of a notification event. The shape
// mirrors server.Notification but lives in the store package to keep the
// dependency direction sane (server imports store, not the other way around).
type Notification struct {
	ID          string
	Kind        string
	Severity    string
	Title       string
	Body        sql.NullString
	ContextJSON sql.NullString
	CreatedAt   int64
	ReadAt      sql.NullInt64
}

// NotificationsRepo persists notifications.
type NotificationsRepo struct{ db *sql.DB }

// NewNotificationsRepo constructs a NotificationsRepo.
func NewNotificationsRepo(db *sql.DB) *NotificationsRepo { return &NotificationsRepo{db: db} }

// Insert persists a notification. Idempotent on id (PRIMARY KEY) — duplicate
// ids are a no-op so a buggy producer that double-emits doesn't double-count.
func (r *NotificationsRepo) Insert(ctx context.Context, n Notification) error {
	if n.CreatedAt == 0 {
		n.CreatedAt = time.Now().Unix()
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO notifications
			(id, kind, severity, title, body, context_json, created_at, read_at)
		VALUES (?,?,?,?,?,?,?,?)
	`, n.ID, n.Kind, n.Severity, n.Title, n.Body, n.ContextJSON, n.CreatedAt, n.ReadAt)
	if err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}
	return nil
}

// List returns at most `limit` notifications, newest first. When unreadOnly
// is true, only rows with read_at IS NULL are returned.
func (r *NotificationsRepo) List(ctx context.Context, unreadOnly bool, limit int) ([]Notification, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := `SELECT id, kind, severity, title, body, context_json, created_at, read_at
	      FROM notifications`
	if unreadOnly {
		q += " WHERE read_at IS NULL"
	}
	q += " ORDER BY created_at DESC LIMIT ?"
	rows, err := r.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()
	out := []Notification{}
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.Kind, &n.Severity, &n.Title, &n.Body, &n.ContextJSON, &n.CreatedAt, &n.ReadAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// CountUnread returns the number of notifications with read_at IS NULL.
func (r *NotificationsRepo) CountUnread(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM notifications WHERE read_at IS NULL`).Scan(&n)
	return n, err
}

// MarkRead sets read_at on a single notification. Idempotent — calling it on
// an already-read row is a no-op.
func (r *NotificationsRepo) MarkRead(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE notifications SET read_at=? WHERE id=? AND read_at IS NULL`,
		time.Now().Unix(), id)
	return err
}

// MarkAllRead bumps every unread row in one statement.
func (r *NotificationsRepo) MarkAllRead(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE notifications SET read_at=? WHERE read_at IS NULL`,
		time.Now().Unix())
	return err
}
