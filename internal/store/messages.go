package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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
	Activity      sql.NullString // JSON: thinking + tools used during this turn (assistant only)
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
			engine, model, activity
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	`,
		m.Channel, m.Direction, m.JID, m.Body, m.MediaType, m.MediaPath, m.MediaCaption,
		m.LocationLat, m.LocationLng, m.LocationName, m.QuotedID, m.ReplyTo, m.TS, m.IsRead,
		m.Engine, m.Model, m.Activity,
	)
	if err != nil {
		return 0, fmt.Errorf("insert message: %w", err)
	}
	return res.LastInsertId()
}

// SetActivity attaches an activity blob to an existing message (assistant turns).
func (r *MessagesRepo) SetActivity(ctx context.Context, id int64, activityJSON string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE wa_messages SET activity=? WHERE id=?`, activityJSON, id)
	return err
}

// Recent returns the latest N messages, optionally filtered by channel.
func (r *MessagesRepo) Recent(ctx context.Context, channel string, limit int) ([]Message, error) {
	return r.Range(ctx, channel, 0, limit)
}

// Range returns up to `limit` messages with id < `beforeID` (0 = no upper
// bound, i.e. latest page). Filtered by channel when non-empty. Sorted by
// id desc so the caller can paginate by passing the smallest id back as
// the next `beforeID`.
func (r *MessagesRepo) Range(ctx context.Context, channel string, beforeID int64, limit int) ([]Message, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	q := `SELECT id, channel, direction, jid, body, media_type, media_path, media_caption,
	             location_lat, location_lng, location_name, quoted_id, reply_to, ts, is_read,
	             engine, model, activity
	      FROM wa_messages`
	args := []any{}
	conds := []string{}
	if channel != "" {
		conds = append(conds, "channel = ?")
		args = append(args, channel)
	}
	if beforeID > 0 {
		conds = append(conds, "id < ?")
		args = append(args, beforeID)
	}
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	q += ` ORDER BY id DESC LIMIT ?`
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
			&m.Engine, &m.Model, &m.Activity); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Search runs a FTS5 query over wa_messages_fts and returns the hits ordered
// by recency (latest first). Channel filter is optional.
func (r *MessagesRepo) Search(ctx context.Context, channel, query string, limit int) ([]Message, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return []Message{}, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	sql := `SELECT m.id, m.channel, m.direction, m.jid, m.body, m.media_type, m.media_path, m.media_caption,
	               m.location_lat, m.location_lng, m.location_name, m.quoted_id, m.reply_to, m.ts, m.is_read,
	               m.engine, m.model, m.activity
	        FROM wa_messages_fts f
	        JOIN wa_messages m ON m.id = f.rowid
	        WHERE wa_messages_fts MATCH ?`
	args := []any{q}
	if channel != "" {
		sql += ` AND m.channel = ?`
		args = append(args, channel)
	}
	sql += ` ORDER BY m.ts DESC LIMIT ?`
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Channel, &m.Direction, &m.JID, &m.Body, &m.MediaType, &m.MediaPath, &m.MediaCaption,
			&m.LocationLat, &m.LocationLng, &m.LocationName, &m.QuotedID, &m.ReplyTo, &m.TS, &m.IsRead,
			&m.Engine, &m.Model, &m.Activity); err != nil {
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
