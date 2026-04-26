package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Diagram stores an editable Excalidraw scene plus its Mermaid source.
type Diagram struct {
	ID             int64
	ProjectID      sql.NullInt64
	Title          string
	Prompt         sql.NullString
	MermaidSource  sql.NullString
	ExcalidrawJSON string
	CreatedAt      int64
	UpdatedAt      int64
}

// DiagramsRepo persists diagrams.
type DiagramsRepo struct{ db *sql.DB }

func NewDiagramsRepo(db *sql.DB) *DiagramsRepo { return &DiagramsRepo{db: db} }

// List returns diagrams sorted by updated_at desc, optionally scoped to a project.
func (r *DiagramsRepo) List(ctx context.Context, projectID *int64) ([]Diagram, error) {
	q := `SELECT id, project_id, title, prompt, mermaid_source, excalidraw_json, created_at, updated_at
	      FROM diagrams`
	args := []any{}
	if projectID != nil {
		q += ` WHERE project_id = ?`
		args = append(args, *projectID)
	}
	q += ` ORDER BY updated_at DESC, id DESC`
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list diagrams: %w", err)
	}
	defer rows.Close()
	out := []Diagram{}
	for rows.Next() {
		var d Diagram
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.Title, &d.Prompt, &d.MermaidSource, &d.ExcalidrawJSON, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Create inserts a diagram.
func (r *DiagramsRepo) Create(ctx context.Context, d Diagram) (int64, error) {
	now := time.Now().Unix()
	if d.CreatedAt == 0 {
		d.CreatedAt = now
	}
	d.UpdatedAt = now
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO diagrams(project_id, title, prompt, mermaid_source, excalidraw_json, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?)
	`, d.ProjectID, d.Title, d.Prompt, d.MermaidSource, d.ExcalidrawJSON, d.CreatedAt, d.UpdatedAt)
	if err != nil {
		return 0, fmt.Errorf("insert diagram: %w", err)
	}
	return res.LastInsertId()
}

// Get returns one diagram by id.
func (r *DiagramsRepo) Get(ctx context.Context, id int64) (*Diagram, error) {
	var d Diagram
	err := r.db.QueryRowContext(ctx, `
		SELECT id, project_id, title, prompt, mermaid_source, excalidraw_json, created_at, updated_at
		FROM diagrams WHERE id = ?
	`, id).Scan(&d.ID, &d.ProjectID, &d.Title, &d.Prompt, &d.MermaidSource, &d.ExcalidrawJSON, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// Update replaces editable fields and bumps updated_at.
func (r *DiagramsRepo) Update(ctx context.Context, d Diagram) error {
	d.UpdatedAt = time.Now().Unix()
	res, err := r.db.ExecContext(ctx, `
		UPDATE diagrams
		SET project_id=?, title=?, prompt=?, mermaid_source=?, excalidraw_json=?, updated_at=?
		WHERE id=?
	`, d.ProjectID, d.Title, d.Prompt, d.MermaidSource, d.ExcalidrawJSON, d.UpdatedAt, d.ID)
	if err != nil {
		return fmt.Errorf("update diagram: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// Delete removes a diagram.
func (r *DiagramsRepo) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM diagrams WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
