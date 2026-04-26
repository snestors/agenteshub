package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Agent is a mini-agent definition (persistent, scheduled).
type Agent struct {
	ID           int64
	Name         string
	Description  sql.NullString
	SystemPrompt string
	Engine       string
	Enabled      bool
	CreatedBy    sql.NullString
	ProjectID    sql.NullInt64
	CreatedAt    int64
	UpdatedAt    int64
}

// AgentSchedule is a cron entry attached to an agent.
type AgentSchedule struct {
	ID             int64
	AgentID        int64
	CronExpr       string
	PromptTemplate string
	NotifyTarget   string
	Enabled        bool
	LastRunAt      sql.NullInt64
	NextRun        int64
}

// AgentRun is a single execution of a mini-agent.
type AgentRun struct {
	ID         int64
	AgentID    int64
	ScheduleID sql.NullInt64
	Trigger    string
	StartedAt  int64
	FinishedAt sql.NullInt64
	Status     string
	Prompt     string
	Result     sql.NullString
	ToolsUsed  sql.NullString
	CostTokens int64
	Error      sql.NullString
}

// AgentsRepo persists agents + schedules + runs + sessions.
type AgentsRepo struct{ db *sql.DB }

func NewAgentsRepo(db *sql.DB) *AgentsRepo { return &AgentsRepo{db: db} }

// List returns every agent.
func (r *AgentsRepo) List(ctx context.Context) ([]Agent, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, description, system_prompt, engine, enabled, created_by, project_id, created_at, updated_at
		FROM agents ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer rows.Close()
	out := []Agent{}
	for rows.Next() {
		var a Agent
		var en int
		if err := rows.Scan(&a.ID, &a.Name, &a.Description, &a.SystemPrompt, &a.Engine, &en, &a.CreatedBy, &a.ProjectID, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		a.Enabled = en == 1
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetByName looks up an agent by name.
func (r *AgentsRepo) GetByName(ctx context.Context, name string) (*Agent, error) {
	var a Agent
	var en int
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, description, system_prompt, engine, enabled, created_by, project_id, created_at, updated_at
		FROM agents WHERE name = ?
	`, name).Scan(&a.ID, &a.Name, &a.Description, &a.SystemPrompt, &a.Engine, &en, &a.CreatedBy, &a.ProjectID, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	a.Enabled = en == 1
	return &a, nil
}

// Create inserts a new agent.
func (r *AgentsRepo) Create(ctx context.Context, a Agent) (int64, error) {
	now := time.Now().Unix()
	if a.CreatedAt == 0 {
		a.CreatedAt = now
	}
	a.UpdatedAt = now
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO agents(name, description, system_prompt, engine, enabled, created_by, project_id, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?)
	`, a.Name, a.Description, a.SystemPrompt, a.Engine, boolToInt(a.Enabled), a.CreatedBy, a.ProjectID, a.CreatedAt, a.UpdatedAt)
	if err != nil {
		return 0, fmt.Errorf("insert agent: %w", err)
	}
	return res.LastInsertId()
}

// SetEnabled toggles enabled flag.
func (r *AgentsRepo) SetEnabled(ctx context.Context, id int64, enabled bool) error {
	_, err := r.db.ExecContext(ctx, `UPDATE agents SET enabled=?, updated_at=? WHERE id=?`, boolToInt(enabled), time.Now().Unix(), id)
	return err
}

// AddSchedule attaches a cron schedule to an agent.
func (r *AgentsRepo) AddSchedule(ctx context.Context, s AgentSchedule) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO agent_schedules(agent_id, cron_expr, prompt_template, notify_target, enabled, next_run)
		VALUES(?,?,?,?,?,?)
	`, s.AgentID, s.CronExpr, s.PromptTemplate, s.NotifyTarget, boolToInt(s.Enabled), s.NextRun)
	if err != nil {
		return 0, fmt.Errorf("insert schedule: %w", err)
	}
	return res.LastInsertId()
}

// PendingSchedules returns all enabled schedules whose next_run <= now.
func (r *AgentsRepo) PendingSchedules(ctx context.Context, now int64) ([]AgentSchedule, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, agent_id, cron_expr, prompt_template, notify_target, enabled, last_run_at, next_run
		FROM agent_schedules WHERE enabled=1 AND next_run <= ?
	`, now)
	if err != nil {
		return nil, fmt.Errorf("query pending: %w", err)
	}
	defer rows.Close()
	out := []AgentSchedule{}
	for rows.Next() {
		var s AgentSchedule
		var en int
		if err := rows.Scan(&s.ID, &s.AgentID, &s.CronExpr, &s.PromptTemplate, &s.NotifyTarget, &en, &s.LastRunAt, &s.NextRun); err != nil {
			return nil, err
		}
		s.Enabled = en == 1
		out = append(out, s)
	}
	return out, rows.Err()
}

// SetNextRun updates the next_run timestamp of a schedule after firing it.
func (r *AgentsRepo) SetNextRun(ctx context.Context, scheduleID int64, lastRun, nextRun int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE agent_schedules SET last_run_at=?, next_run=? WHERE id=?`, lastRun, nextRun, scheduleID)
	return err
}

// InsertRun records a new mini-agent run.
func (r *AgentsRepo) InsertRun(ctx context.Context, run AgentRun) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO agent_runs(agent_id, schedule_id, trigger, started_at, finished_at, status, prompt, result, tools_used, cost_tokens, error)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)
	`, run.AgentID, run.ScheduleID, run.Trigger, run.StartedAt, run.FinishedAt, run.Status, run.Prompt, run.Result, run.ToolsUsed, run.CostTokens, run.Error)
	if err != nil {
		return 0, fmt.Errorf("insert run: %w", err)
	}
	return res.LastInsertId()
}

// FinishRun finalizes an agent run.
func (r *AgentsRepo) FinishRun(ctx context.Context, id int64, status, result string, tokens int64, errStr string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE agent_runs SET finished_at=?, status=?, result=?, cost_tokens=?, error=? WHERE id=?
	`, time.Now().Unix(), status, result, tokens, sql.NullString{String: errStr, Valid: errStr != ""}, id)
	return err
}

// RunsForAgent returns recent runs of an agent.
func (r *AgentsRepo) RunsForAgent(ctx context.Context, agentID int64, limit int) ([]AgentRun, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, agent_id, schedule_id, trigger, started_at, finished_at, status, prompt, result, tools_used, cost_tokens, error
		FROM agent_runs WHERE agent_id=? ORDER BY started_at DESC LIMIT ?
	`, agentID, limit)
	if err != nil {
		return nil, fmt.Errorf("query runs: %w", err)
	}
	defer rows.Close()
	out := []AgentRun{}
	for rows.Next() {
		var r AgentRun
		if err := rows.Scan(&r.ID, &r.AgentID, &r.ScheduleID, &r.Trigger, &r.StartedAt, &r.FinishedAt, &r.Status, &r.Prompt, &r.Result, &r.ToolsUsed, &r.CostTokens, &r.Error); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
