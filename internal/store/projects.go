package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Project is a coding workspace accessible from the browser.
type Project struct {
	ID            int64
	Name          string
	Path          string
	Description   sql.NullString
	DefaultEngine string
	CreatedAt     int64
	UpdatedAt     int64
}

// ProjectSession is a single Claude/Codex resume session inside a project.
type ProjectSession struct {
	ID           int64
	ProjectID    int64
	Name         string
	SessionID    string
	Engine       string
	Summary      sql.NullString
	LastActiveAt sql.NullInt64
	CreatedAt    int64
}

// ProjectsRepo persists projects + project_sessions.
type ProjectsRepo struct{ db *sql.DB }

func NewProjectsRepo(db *sql.DB) *ProjectsRepo { return &ProjectsRepo{db: db} }

// List returns every project sorted by updated_at desc.
func (r *ProjectsRepo) List(ctx context.Context) ([]Project, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, path, description, default_engine, created_at, updated_at
		FROM projects ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()
	out := []Project{}
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Path, &p.Description, &p.DefaultEngine, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetByName looks up a project by name.
func (r *ProjectsRepo) GetByName(ctx context.Context, name string) (*Project, error) {
	var p Project
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, path, description, default_engine, created_at, updated_at
		FROM projects WHERE name = ?
	`, name).Scan(&p.ID, &p.Name, &p.Path, &p.Description, &p.DefaultEngine, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// Create inserts a new project.
func (r *ProjectsRepo) Create(ctx context.Context, p Project) (int64, error) {
	now := time.Now().Unix()
	if p.CreatedAt == 0 {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO projects(name, path, description, default_engine, created_at, updated_at)
		VALUES(?,?,?,?,?,?)
	`, p.Name, p.Path, p.Description, p.DefaultEngine, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return 0, fmt.Errorf("insert project: %w", err)
	}
	return res.LastInsertId()
}

// ListSessions returns all sessions of a project.
func (r *ProjectsRepo) ListSessions(ctx context.Context, projectID int64) ([]ProjectSession, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, project_id, name, session_id, engine, summary, last_active_at, created_at
		FROM project_sessions WHERE project_id = ? ORDER BY last_active_at DESC
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()
	out := []ProjectSession{}
	for rows.Next() {
		var s ProjectSession
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.Name, &s.SessionID, &s.Engine, &s.Summary, &s.LastActiveAt, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// CreateSession adds a project session.
func (r *ProjectsRepo) CreateSession(ctx context.Context, s ProjectSession) (int64, error) {
	if s.CreatedAt == 0 {
		s.CreatedAt = time.Now().Unix()
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO project_sessions(project_id, name, session_id, engine, summary, last_active_at, created_at)
		VALUES(?,?,?,?,?,?,?)
	`, s.ProjectID, s.Name, s.SessionID, s.Engine, s.Summary, s.LastActiveAt, s.CreatedAt)
	if err != nil {
		return 0, fmt.Errorf("insert project session: %w", err)
	}
	return res.LastInsertId()
}
