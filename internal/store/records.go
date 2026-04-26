package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Record is a row in agent_records (schema-on-read free JSON).
type Record struct {
	ID        int64
	Topic     string
	Data      string // JSON
	AgentName sql.NullString
	Source    string
	CreatedAt int64
}

// RecordsRepo persists agent_records.
type RecordsRepo struct{ db *sql.DB }

func NewRecordsRepo(db *sql.DB) *RecordsRepo { return &RecordsRepo{db: db} }

// Insert adds a record.
func (r *RecordsRepo) Insert(ctx context.Context, rec Record) (int64, error) {
	if rec.CreatedAt == 0 {
		rec.CreatedAt = time.Now().Unix()
	}
	if rec.Source == "" {
		rec.Source = "agent"
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO agent_records(topic, data, agent_name, source, created_at)
		VALUES(?,?,?,?,?)
	`, rec.Topic, rec.Data, rec.AgentName, rec.Source, rec.CreatedAt)
	if err != nil {
		return 0, fmt.Errorf("insert record: %w", err)
	}
	return res.LastInsertId()
}

// Query returns recent records for a topic.
func (r *RecordsRepo) Query(ctx context.Context, topic string, since int64, limit int) ([]Record, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	args := []any{topic}
	q := `SELECT id, topic, data, agent_name, source, created_at FROM agent_records WHERE topic = ?`
	if since > 0 {
		q += ` AND created_at >= ?`
		args = append(args, since)
	}
	q += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query records: %w", err)
	}
	defer rows.Close()
	out := []Record{}
	for rows.Next() {
		var rec Record
		if err := rows.Scan(&rec.ID, &rec.Topic, &rec.Data, &rec.AgentName, &rec.Source, &rec.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// ListTopics returns all distinct topics with their counts.
func (r *RecordsRepo) ListTopics(ctx context.Context) ([]struct {
	Topic string
	Count int
}, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT topic, COUNT(*) FROM agent_records GROUP BY topic ORDER BY 2 DESC`)
	if err != nil {
		return nil, fmt.Errorf("list topics: %w", err)
	}
	defer rows.Close()
	out := []struct {
		Topic string
		Count int
	}{}
	for rows.Next() {
		var t string
		var c int
		if err := rows.Scan(&t, &c); err != nil {
			return nil, err
		}
		out = append(out, struct {
			Topic string
			Count int
		}{t, c})
	}
	return out, rows.Err()
}

// DeleteTopic removes all records of a topic.
func (r *RecordsRepo) DeleteTopic(ctx context.Context, topic string) (int64, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM agent_records WHERE topic = ?`, topic)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
