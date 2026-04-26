package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// OpenSpecChange tracks one gated SDD/OpenSpec change for a project.
type OpenSpecChange struct {
	ID           int64
	ProjectID    int64
	Name         string
	Description  sql.NullString
	State        string
	CurrentPhase string
	Feedback     sql.NullString
	CreatedAt    int64
	UpdatedAt    int64
	ArchivedAt   sql.NullInt64
}

// OpenSpecChangesRepo persists OpenSpec gated changes.
type OpenSpecChangesRepo struct{ db *sql.DB }

func NewOpenSpecChangesRepo(db *sql.DB) *OpenSpecChangesRepo { return &OpenSpecChangesRepo{db: db} }

// Create inserts a new OpenSpec change in pending_proposal.
func (r *OpenSpecChangesRepo) Create(ctx context.Context, c OpenSpecChange) (int64, error) {
	now := time.Now().Unix()
	if c.CreatedAt == 0 {
		c.CreatedAt = now
	}
	if c.UpdatedAt == 0 {
		c.UpdatedAt = now
	}
	if c.State == "" {
		c.State = "pending_proposal"
	}
	if c.CurrentPhase == "" {
		c.CurrentPhase = "proposal"
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO openspec_changes(project_id, name, description, state, current_phase, feedback, created_at, updated_at, archived_at)
		VALUES(?,?,?,?,?,?,?,?,?)
	`, c.ProjectID, c.Name, c.Description, c.State, c.CurrentPhase, c.Feedback, c.CreatedAt, c.UpdatedAt, c.ArchivedAt)
	if err != nil {
		return 0, fmt.Errorf("insert openspec change: %w", err)
	}
	return res.LastInsertId()
}

// GetByID returns a change by id.
func (r *OpenSpecChangesRepo) GetByID(ctx context.Context, id int64) (*OpenSpecChange, error) {
	var c OpenSpecChange
	err := r.db.QueryRowContext(ctx, `
		SELECT id, project_id, name, description, state, current_phase, feedback, created_at, updated_at, archived_at
		FROM openspec_changes WHERE id = ?
	`, id).Scan(&c.ID, &c.ProjectID, &c.Name, &c.Description, &c.State, &c.CurrentPhase, &c.Feedback, &c.CreatedAt, &c.UpdatedAt, &c.ArchivedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// GetByProjectAndName returns a change scoped by project + slug.
func (r *OpenSpecChangesRepo) GetByProjectAndName(ctx context.Context, projectID int64, name string) (*OpenSpecChange, error) {
	var c OpenSpecChange
	err := r.db.QueryRowContext(ctx, `
		SELECT id, project_id, name, description, state, current_phase, feedback, created_at, updated_at, archived_at
		FROM openspec_changes WHERE project_id = ? AND name = ?
	`, projectID, name).Scan(&c.ID, &c.ProjectID, &c.Name, &c.Description, &c.State, &c.CurrentPhase, &c.Feedback, &c.CreatedAt, &c.UpdatedAt, &c.ArchivedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ListByProject returns changes sorted newest first.
func (r *OpenSpecChangesRepo) ListByProject(ctx context.Context, projectID int64) ([]OpenSpecChange, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, project_id, name, description, state, current_phase, feedback, created_at, updated_at, archived_at
		FROM openspec_changes WHERE project_id = ? ORDER BY updated_at DESC, id DESC
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list openspec changes: %w", err)
	}
	defer rows.Close()
	out := []OpenSpecChange{}
	for rows.Next() {
		var c OpenSpecChange
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.Name, &c.Description, &c.State, &c.CurrentPhase, &c.Feedback, &c.CreatedAt, &c.UpdatedAt, &c.ArchivedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpdateState changes state/current_phase and clears feedback when requested by caller.
func (r *OpenSpecChangesRepo) UpdateState(ctx context.Context, id int64, state, currentPhase string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE openspec_changes SET state=?, current_phase=?, updated_at=unixepoch(), feedback=NULL WHERE id=?
	`, state, currentPhase, id)
	if err != nil {
		return fmt.Errorf("update openspec state: %w", err)
	}
	return rowsAffectedOrNoRows(res)
}

// SetFeedback stores the latest free-form feedback for regeneration.
func (r *OpenSpecChangesRepo) SetFeedback(ctx context.Context, id int64, feedback string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE openspec_changes SET feedback=?, updated_at=unixepoch() WHERE id=?
	`, sql.NullString{String: feedback, Valid: feedback != ""}, id)
	if err != nil {
		return fmt.Errorf("set openspec feedback: %w", err)
	}
	return rowsAffectedOrNoRows(res)
}

// Archive marks a change archived with archived_at set.
func (r *OpenSpecChangesRepo) Archive(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE openspec_changes
		SET state='archived', current_phase='archive', feedback=NULL, archived_at=unixepoch(), updated_at=unixepoch()
		WHERE id=?
	`, id)
	if err != nil {
		return fmt.Errorf("archive openspec change: %w", err)
	}
	return rowsAffectedOrNoRows(res)
}

func rowsAffectedOrNoRows(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
