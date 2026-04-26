package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// WaOutboxItem is an outgoing WhatsApp message queued by the MCP subprocess
// for the daemon's outbox worker to dispatch.
type WaOutboxItem struct {
	ID               int64
	JID              string
	Kind             string // 'text'|'image'|'voice'|'audio'|'document'|'video'|'location'
	Body             sql.NullString
	MediaPath        sql.NullString
	Caption          sql.NullString
	LocLat           sql.NullFloat64
	LocLng           sql.NullFloat64
	LocName          sql.NullString
	ReplyTo          sql.NullString // WA stanza id we are quoting
	ReplyParticipant sql.NullString // JID of the original sender (for groups)
	Status           string
	Error            sql.NullString
	Attempts         int
	CreatedAt        int64
	SentAt           sql.NullInt64
}

type WaOutboxRepo struct{ db *sql.DB }

func NewWaOutboxRepo(db *sql.DB) *WaOutboxRepo { return &WaOutboxRepo{db: db} }

// Enqueue inserts a pending item and returns its id. CreatedAt is filled if zero.
func (r *WaOutboxRepo) Enqueue(ctx context.Context, item WaOutboxItem) (int64, error) {
	if item.CreatedAt == 0 {
		item.CreatedAt = time.Now().Unix()
	}
	if item.Status == "" {
		item.Status = "pending"
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO wa_outbox(jid, kind, body, media_path, caption, loc_lat, loc_lng, loc_name, reply_to, reply_participant, status, created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?)
	`, item.JID, item.Kind, item.Body, item.MediaPath, item.Caption,
		item.LocLat, item.LocLng, item.LocName, item.ReplyTo, item.ReplyParticipant,
		item.Status, item.CreatedAt)
	if err != nil {
		return 0, fmt.Errorf("enqueue wa_outbox: %w", err)
	}
	return res.LastInsertId()
}

// PullPending returns up to `limit` pending items, oldest first.
func (r *WaOutboxRepo) PullPending(ctx context.Context, limit int) ([]WaOutboxItem, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, jid, kind, body, media_path, caption, loc_lat, loc_lng, loc_name, reply_to, reply_participant, status, error, attempts, created_at, sent_at
		FROM wa_outbox WHERE status = 'pending' ORDER BY created_at ASC LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query wa_outbox: %w", err)
	}
	defer rows.Close()
	out := []WaOutboxItem{}
	for rows.Next() {
		var x WaOutboxItem
		if err := rows.Scan(&x.ID, &x.JID, &x.Kind, &x.Body, &x.MediaPath, &x.Caption,
			&x.LocLat, &x.LocLng, &x.LocName, &x.ReplyTo, &x.ReplyParticipant,
			&x.Status, &x.Error, &x.Attempts, &x.CreatedAt, &x.SentAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// MarkSent flips an item to status='sent' with sent_at = now.
func (r *WaOutboxRepo) MarkSent(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE wa_outbox SET status='sent', sent_at=?, error=NULL WHERE id=?
	`, time.Now().Unix(), id)
	return err
}

// MarkError flips an item to status='error' with the given message + bumps attempts.
func (r *WaOutboxRepo) MarkError(ctx context.Context, id int64, msg string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE wa_outbox SET status='error', error=?, attempts=attempts+1 WHERE id=?
	`, msg, id)
	return err
}

// GetByID returns one item or nil if not found.
func (r *WaOutboxRepo) GetByID(ctx context.Context, id int64) (*WaOutboxItem, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, jid, kind, body, media_path, caption, loc_lat, loc_lng, loc_name, reply_to, reply_participant, status, error, attempts, created_at, sent_at
		FROM wa_outbox WHERE id = ?
	`, id)
	var x WaOutboxItem
	if err := row.Scan(&x.ID, &x.JID, &x.Kind, &x.Body, &x.MediaPath, &x.Caption,
		&x.LocLat, &x.LocLng, &x.LocName, &x.ReplyTo, &x.ReplyParticipant,
		&x.Status, &x.Error, &x.Attempts, &x.CreatedAt, &x.SentAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &x, nil
}
