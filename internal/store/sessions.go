package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SessionMessage is a turn persisted as human-legible text (capa 3).
type SessionMessage struct {
	ID            int64
	Scope         string // 'topic'|'project'|'agent'|'main'
	TopicID       sql.NullInt64
	ProjectID     sql.NullInt64
	ProjectSessID sql.NullInt64
	SessionID     string
	Role          string // 'user'|'assistant'|'tool'|'system'
	Body          sql.NullString
	ToolName      sql.NullString
	ToolArgs      sql.NullString
	ToolResult    sql.NullString
	CostTokens    int64
	TS            int64
}

// AgentSession links an agent name to its current resume id.
type AgentSession struct {
	AgentName string
	Engine    string
	SessionID string
	UpdatedAt int64
}

// SessionsRepo persists session_messages + agent_sessions.
type SessionsRepo struct{ db *sql.DB }

func NewSessionsRepo(db *sql.DB) *SessionsRepo { return &SessionsRepo{db: db} }

// AppendMessage stores a session turn.
func (r *SessionsRepo) AppendMessage(ctx context.Context, m SessionMessage) (int64, error) {
	if m.TS == 0 {
		m.TS = time.Now().Unix()
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO session_messages(scope, topic_id, project_id, project_sess_id, session_id, role, body, tool_name, tool_args, tool_result, cost_tokens, ts)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?)
	`, m.Scope, m.TopicID, m.ProjectID, m.ProjectSessID, m.SessionID, m.Role, m.Body, m.ToolName, m.ToolArgs, m.ToolResult, m.CostTokens, m.TS)
	if err != nil {
		return 0, fmt.Errorf("append message: %w", err)
	}
	return res.LastInsertId()
}

// MessagesForSession returns turns of a session ordered by ts.
func (r *SessionsRepo) MessagesForSession(ctx context.Context, sessionID string, limit int) ([]SessionMessage, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, scope, topic_id, project_id, project_sess_id, session_id, role, body, tool_name, tool_args, tool_result, cost_tokens, ts
		FROM session_messages WHERE session_id = ? ORDER BY ts ASC LIMIT ?
	`, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()
	out := []SessionMessage{}
	for rows.Next() {
		var m SessionMessage
		if err := rows.Scan(&m.ID, &m.Scope, &m.TopicID, &m.ProjectID, &m.ProjectSessID, &m.SessionID, &m.Role, &m.Body, &m.ToolName, &m.ToolArgs, &m.ToolResult, &m.CostTokens, &m.TS); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// GetAgentSession returns the resume id for a named agent.
func (r *SessionsRepo) GetAgentSession(ctx context.Context, name string) (*AgentSession, error) {
	var s AgentSession
	err := r.db.QueryRowContext(ctx, `SELECT agent_name, engine, session_id, updated_at FROM agent_sessions WHERE agent_name = ?`, name).Scan(&s.AgentName, &s.Engine, &s.SessionID, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// UpsertAgentSession stores the latest resume id for an agent.
func (r *SessionsRepo) UpsertAgentSession(ctx context.Context, s AgentSession) error {
	if s.UpdatedAt == 0 {
		s.UpdatedAt = time.Now().Unix()
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO agent_sessions(agent_name, engine, session_id, updated_at) VALUES(?,?,?,?)
		ON CONFLICT(agent_name) DO UPDATE SET engine=excluded.engine, session_id=excluded.session_id, updated_at=excluded.updated_at
	`, s.AgentName, s.Engine, s.SessionID, s.UpdatedAt)
	return err
}
