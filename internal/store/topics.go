package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Topic is a conversational context tracked by the main agent.
type Topic struct {
	ID           int64
	Name         string
	Description  sql.NullString
	Keywords     sql.NullString // JSON array
	ProjectID    sql.NullInt64
	SessionID    string
	Engine       string
	IsDefault    bool
	LastActiveAt sql.NullInt64
	CreatedAt    int64
}

// TopicState is the JSON-structured memory the main reads instead of the raw JSONL.
type TopicState struct {
	TopicID         int64
	Headline        sql.NullString
	ActiveIssues    sql.NullString
	RecentDecisions sql.NullString
	Pending         sql.NullString
	NextActionHint  sql.NullString
	LastEventAt     sql.NullInt64
	UpdatedAt       int64
}

// TopicsRepo persists topics + topic_state.
type TopicsRepo struct{ db *sql.DB }

func NewTopicsRepo(db *sql.DB) *TopicsRepo { return &TopicsRepo{db: db} }

// List returns every topic ordered by last activity.
func (r *TopicsRepo) List(ctx context.Context) ([]Topic, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, description, keywords, project_id, session_id, engine, is_default, last_active_at, created_at
		FROM topics ORDER BY COALESCE(last_active_at, 0) DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list topics: %w", err)
	}
	defer rows.Close()
	out := []Topic{}
	for rows.Next() {
		var t Topic
		var d int
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Keywords, &t.ProjectID, &t.SessionID, &t.Engine, &d, &t.LastActiveAt, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan topic: %w", err)
		}
		t.IsDefault = d == 1
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetByName returns a topic by name (or sql.ErrNoRows).
func (r *TopicsRepo) GetByName(ctx context.Context, name string) (*Topic, error) {
	var t Topic
	var d int
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, description, keywords, project_id, session_id, engine, is_default, last_active_at, created_at
		FROM topics WHERE name = ?
	`, name).Scan(&t.ID, &t.Name, &t.Description, &t.Keywords, &t.ProjectID, &t.SessionID, &t.Engine, &d, &t.LastActiveAt, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	t.IsDefault = d == 1
	return &t, nil
}

// Create inserts a new topic.
func (r *TopicsRepo) Create(ctx context.Context, t Topic) (int64, error) {
	if t.CreatedAt == 0 {
		t.CreatedAt = time.Now().Unix()
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO topics(name, description, keywords, project_id, session_id, engine, is_default, last_active_at, created_at)
		VALUES(?,?,?,?,?,?,?,?,?)
	`, t.Name, t.Description, t.Keywords, t.ProjectID, t.SessionID, t.Engine,
		boolToInt(t.IsDefault), t.LastActiveAt, t.CreatedAt)
	if err != nil {
		return 0, fmt.Errorf("insert topic: %w", err)
	}
	return res.LastInsertId()
}

// Touch updates the last_active_at of a topic.
func (r *TopicsRepo) Touch(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE topics SET last_active_at=? WHERE id=?`, time.Now().Unix(), id)
	return err
}

// SetSessionID updates the resume id of the topic (after first run).
func (r *TopicsRepo) SetSessionID(ctx context.Context, id int64, sessionID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE topics SET session_id=? WHERE id=?`, sessionID, id)
	return err
}

// GetState loads the JSON state for a topic.
func (r *TopicsRepo) GetState(ctx context.Context, topicID int64) (*TopicState, error) {
	var s TopicState
	err := r.db.QueryRowContext(ctx, `
		SELECT topic_id, headline, active_issues, recent_decisions, pending, next_action_hint, last_event_at, updated_at
		FROM topic_state WHERE topic_id = ?
	`, topicID).Scan(&s.TopicID, &s.Headline, &s.ActiveIssues, &s.RecentDecisions, &s.Pending, &s.NextActionHint, &s.LastEventAt, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// UpsertState stores topic state (full overwrite).
func (r *TopicsRepo) UpsertState(ctx context.Context, s TopicState) error {
	if s.UpdatedAt == 0 {
		s.UpdatedAt = time.Now().Unix()
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO topic_state(topic_id, headline, active_issues, recent_decisions, pending, next_action_hint, last_event_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?)
		ON CONFLICT(topic_id) DO UPDATE SET
			headline=excluded.headline,
			active_issues=excluded.active_issues,
			recent_decisions=excluded.recent_decisions,
			pending=excluded.pending,
			next_action_hint=excluded.next_action_hint,
			last_event_at=excluded.last_event_at,
			updated_at=excluded.updated_at
	`, s.TopicID, s.Headline, s.ActiveIssues, s.RecentDecisions, s.Pending, s.NextActionHint, s.LastEventAt, s.UpdatedAt)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
