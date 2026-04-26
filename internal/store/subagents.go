package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SubagentRun is an ephemeral Agent-tool invocation captured from stream-json.
type SubagentRun struct {
	ID                       int64
	ParentSessionID          string
	ParentScope              string
	ParentTopicID            sql.NullInt64
	ParentProjectSessID      sql.NullInt64
	AgentType                sql.NullString
	Description              sql.NullString
	Prompt                   sql.NullString
	Result                   sql.NullString
	Status                   string
	StartedAt                int64
	FinishedAt               sql.NullInt64
	CostTokens               int64
	ToolsUsed                sql.NullString
	WorktreePath             sql.NullString
}

// SubagentRunsRepo persists subagent_runs.
type SubagentRunsRepo struct{ db *sql.DB }

func NewSubagentRunsRepo(db *sql.DB) *SubagentRunsRepo { return &SubagentRunsRepo{db: db} }

// Start inserts a new subagent run with status='running'.
func (r *SubagentRunsRepo) Start(ctx context.Context, run SubagentRun) (int64, error) {
	if run.StartedAt == 0 {
		run.StartedAt = time.Now().Unix()
	}
	if run.Status == "" {
		run.Status = "running"
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO subagent_runs(parent_session_id, parent_scope, parent_topic_id, parent_project_session_id, agent_type, description, prompt, status, started_at)
		VALUES(?,?,?,?,?,?,?,?,?)
	`, run.ParentSessionID, run.ParentScope, run.ParentTopicID, run.ParentProjectSessID, run.AgentType, run.Description, run.Prompt, run.Status, run.StartedAt)
	if err != nil {
		return 0, fmt.Errorf("start subagent run: %w", err)
	}
	return res.LastInsertId()
}

// Finish marks a subagent run as ok/error/cancelled with its result.
func (r *SubagentRunsRepo) Finish(ctx context.Context, id int64, status, result string, tokens int64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE subagent_runs SET finished_at=?, status=?, result=?, cost_tokens=? WHERE id=?
	`, time.Now().Unix(), status, result, tokens, id)
	return err
}

// ByParent returns subagent runs of a parent session.
func (r *SubagentRunsRepo) ByParent(ctx context.Context, parentSessionID string, limit int) ([]SubagentRun, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, parent_session_id, parent_scope, parent_topic_id, parent_project_session_id, agent_type, description, prompt, result, status, started_at, finished_at, cost_tokens, tools_used, worktree_path
		FROM subagent_runs WHERE parent_session_id=? ORDER BY started_at DESC LIMIT ?
	`, parentSessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("query subagents: %w", err)
	}
	defer rows.Close()
	out := []SubagentRun{}
	for rows.Next() {
		var s SubagentRun
		if err := rows.Scan(&s.ID, &s.ParentSessionID, &s.ParentScope, &s.ParentTopicID, &s.ParentProjectSessID, &s.AgentType, &s.Description, &s.Prompt, &s.Result, &s.Status, &s.StartedAt, &s.FinishedAt, &s.CostTokens, &s.ToolsUsed, &s.WorktreePath); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Recent returns the latest subagent runs across all parents.
func (r *SubagentRunsRepo) Recent(ctx context.Context, status string, limit int) ([]SubagentRun, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := `SELECT id, parent_session_id, parent_scope, parent_topic_id, parent_project_session_id, agent_type, description, prompt, result, status, started_at, finished_at, cost_tokens, tools_used, worktree_path
	      FROM subagent_runs`
	args := []any{}
	if status != "" {
		q += ` WHERE status = ?`
		args = append(args, status)
	}
	q += ` ORDER BY started_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query subagents: %w", err)
	}
	defer rows.Close()
	out := []SubagentRun{}
	for rows.Next() {
		var s SubagentRun
		if err := rows.Scan(&s.ID, &s.ParentSessionID, &s.ParentScope, &s.ParentTopicID, &s.ParentProjectSessID, &s.AgentType, &s.Description, &s.Prompt, &s.Result, &s.Status, &s.StartedAt, &s.FinishedAt, &s.CostTokens, &s.ToolsUsed, &s.WorktreePath); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
