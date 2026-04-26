package store

import (
	"context"
	"database/sql"
	"fmt"
)

// Message is a row in wa_messages.
type Message struct {
	ID            int64
	Channel       string // 'wa' | 'web'
	Direction     string // 'in' | 'out'
	JID           sql.NullString
	Body          sql.NullString
	MediaType     sql.NullString
	MediaPath     sql.NullString
	MediaCaption  sql.NullString
	LocationLat   sql.NullFloat64
	LocationLng   sql.NullFloat64
	LocationName  sql.NullString
	QuotedID      sql.NullInt64
	ReplyTo       sql.NullString
	TS            int64
	IsRead        int
	Engine        sql.NullString // engine usado para producir la respuesta (assistant turns)
	Model         sql.NullString // model concreto (sonnet, opus, codex, gemma:2b...)
}

// MessagesRepo persists wa_messages.
type MessagesRepo struct{ db *sql.DB }

func NewMessagesRepo(db *sql.DB) *MessagesRepo { return &MessagesRepo{db: db} }

// Insert adds a new message and returns its id.
func (r *MessagesRepo) Insert(ctx context.Context, m Message) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO wa_messages(
			channel, direction, jid, body, media_type, media_path, media_caption,
			location_lat, location_lng, location_name, quoted_id, reply_to, ts, is_read,
			engine, model
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	`,
		m.Channel, m.Direction, m.JID, m.Body, m.MediaType, m.MediaPath, m.MediaCaption,
		m.LocationLat, m.LocationLng, m.LocationName, m.QuotedID, m.ReplyTo, m.TS, m.IsRead,
		m.Engine, m.Model,
	)
	if err != nil {
		return 0, fmt.Errorf("insert message: %w", err)
	}
	return res.LastInsertId()
}

// Recent returns the latest N messages, optionally filtered by channel.
func (r *MessagesRepo) Recent(ctx context.Context, channel string, limit int) ([]Message, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	q := `SELECT id, channel, direction, jid, body, media_type, media_path, media_caption,
	             location_lat, location_lng, location_name, quoted_id, reply_to, ts, is_read,
	             engine, model
	      FROM wa_messages`
	args := []any{}
	if channel != "" {
		q += ` WHERE channel = ?`
		args = append(args, channel)
	}
	q += ` ORDER BY ts DESC LIMIT ?`
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Channel, &m.Direction, &m.JID, &m.Body, &m.MediaType, &m.MediaPath, &m.MediaCaption,
			&m.LocationLat, &m.LocationLng, &m.LocationName, &m.QuotedID, &m.ReplyTo, &m.TS, &m.IsRead,
			&m.Engine, &m.Model); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// MarkRead sets is_read=1 for the message.
func (r *MessagesRepo) MarkRead(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE wa_messages SET is_read=1 WHERE id=?`, id)
	return err
}
