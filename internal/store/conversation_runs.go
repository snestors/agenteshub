package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type ConversationRun struct {
	ID              int64
	Scope           string
	ScopeKey        string
	Topic           sql.NullString
	Engine          string
	Model           sql.NullString
	SessionID       sql.NullString
	Status          string
	StartedAt       int64
	UpdatedAt       int64
	FinishedAt      sql.NullInt64
	LastError       sql.NullString
	TextPartial     sql.NullString
	ThinkingPartial sql.NullString
	ActivityJSON    sql.NullString
	ResultText      sql.NullString
	LastSeq         int
}

type ConversationRunsRepo struct{ db *sql.DB }

func NewConversationRunsRepo(db *sql.DB) *ConversationRunsRepo { return &ConversationRunsRepo{db: db} }

func (r *ConversationRunsRepo) Upsert(ctx context.Context, run ConversationRun) error {
	if run.StartedAt == 0 {
		run.StartedAt = time.Now().Unix()
	}
	if run.UpdatedAt == 0 {
		run.UpdatedAt = time.Now().Unix()
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO conversation_runs(
			scope, scope_key, topic, engine, model, session_id, status,
			started_at, updated_at, finished_at, last_error,
			text_partial, thinking_partial, activity_json, result_text, last_seq
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(scope, scope_key) DO UPDATE SET
			topic=excluded.topic,
			engine=excluded.engine,
			model=excluded.model,
			session_id=excluded.session_id,
			status=excluded.status,
			started_at=excluded.started_at,
			updated_at=excluded.updated_at,
			finished_at=excluded.finished_at,
			last_error=excluded.last_error,
			text_partial=excluded.text_partial,
			thinking_partial=excluded.thinking_partial,
			activity_json=excluded.activity_json,
			result_text=excluded.result_text,
			last_seq=excluded.last_seq
	`, run.Scope, run.ScopeKey, run.Topic, run.Engine, run.Model, run.SessionID, run.Status,
		run.StartedAt, run.UpdatedAt, run.FinishedAt, run.LastError,
		run.TextPartial, run.ThinkingPartial, run.ActivityJSON, run.ResultText, run.LastSeq)
	if err != nil {
		return fmt.Errorf("upsert conversation run: %w", err)
	}
	return nil
}

func (r *ConversationRunsRepo) Get(ctx context.Context, scope, scopeKey string) (*ConversationRun, error) {
	var run ConversationRun
	err := r.db.QueryRowContext(ctx, `
		SELECT id, scope, scope_key, topic, engine, model, session_id, status,
		       started_at, updated_at, finished_at, last_error,
		       text_partial, thinking_partial, activity_json, result_text, last_seq
		FROM conversation_runs
		WHERE scope=? AND scope_key=?
	`, scope, scopeKey).Scan(
		&run.ID, &run.Scope, &run.ScopeKey, &run.Topic, &run.Engine, &run.Model, &run.SessionID, &run.Status,
		&run.StartedAt, &run.UpdatedAt, &run.FinishedAt, &run.LastError,
		&run.TextPartial, &run.ThinkingPartial, &run.ActivityJSON, &run.ResultText, &run.LastSeq,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get conversation run: %w", err)
	}
	return &run, nil
}

func (r *ConversationRunsRepo) MarkInterruptedOlderThan(ctx context.Context, ts int64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE conversation_runs
		SET status='interrupted', finished_at=COALESCE(finished_at, ?), updated_at=?, last_error=COALESCE(last_error, 'interrupted by daemon restart')
		WHERE status='running' AND updated_at < ?
	`, ts, ts, ts)
	return err
}

func MustJSON(v any) sql.NullString {
	raw, err := json.Marshal(v)
	if err != nil || len(raw) == 0 {
		return sql.NullString{}
	}
	return sql.NullString{String: string(raw), Valid: true}
}
