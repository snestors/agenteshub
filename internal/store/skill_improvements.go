package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SkillImprovement is a proposed change to a skill, awaiting user review.
type SkillImprovement struct {
	ID          int64
	SkillName   string
	Source      string
	ProposedBy  sql.NullString
	Rationale   string
	Patch       string
	PatchKind   string // 'append' | 'replace' | 'diff'
	Status      string // 'pending' | 'approved' | 'rejected'
	ResolvedBy  sql.NullString
	ResolvedAt  sql.NullInt64
	CreatedAt   int64
}

type SkillImprovementsRepo struct{ db *sql.DB }

func NewSkillImprovementsRepo(db *sql.DB) *SkillImprovementsRepo {
	return &SkillImprovementsRepo{db: db}
}

func (r *SkillImprovementsRepo) Create(ctx context.Context, imp SkillImprovement) (int64, error) {
	if imp.CreatedAt == 0 {
		imp.CreatedAt = time.Now().Unix()
	}
	if imp.PatchKind == "" {
		imp.PatchKind = "append"
	}
	if imp.Status == "" {
		imp.Status = "pending"
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO skill_improvements(skill_name, source, proposed_by, rationale, patch, patch_kind, status, created_at)
		VALUES(?,?,?,?,?,?,?,?)
	`, imp.SkillName, imp.Source, imp.ProposedBy, imp.Rationale, imp.Patch, imp.PatchKind, imp.Status, imp.CreatedAt)
	if err != nil {
		return 0, fmt.Errorf("create skill improvement: %w", err)
	}
	return res.LastInsertId()
}

// List returns improvements filtered by status (empty = all). Newest first.
func (r *SkillImprovementsRepo) List(ctx context.Context, status string, limit int) ([]SkillImprovement, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := `SELECT id, skill_name, source, proposed_by, rationale, patch, patch_kind, status, resolved_by, resolved_at, created_at
	      FROM skill_improvements`
	args := []any{}
	if status != "" {
		q += ` WHERE status = ?`
		args = append(args, status)
	}
	q += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list skill improvements: %w", err)
	}
	defer rows.Close()
	out := []SkillImprovement{}
	for rows.Next() {
		var x SkillImprovement
		if err := rows.Scan(&x.ID, &x.SkillName, &x.Source, &x.ProposedBy, &x.Rationale, &x.Patch, &x.PatchKind, &x.Status, &x.ResolvedBy, &x.ResolvedAt, &x.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (r *SkillImprovementsRepo) GetByID(ctx context.Context, id int64) (*SkillImprovement, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, skill_name, source, proposed_by, rationale, patch, patch_kind, status, resolved_by, resolved_at, created_at
		FROM skill_improvements WHERE id = ?
	`, id)
	var x SkillImprovement
	if err := row.Scan(&x.ID, &x.SkillName, &x.Source, &x.ProposedBy, &x.Rationale, &x.Patch, &x.PatchKind, &x.Status, &x.ResolvedBy, &x.ResolvedAt, &x.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &x, nil
}

// Resolve flips status to 'approved' or 'rejected' and records the resolver.
func (r *SkillImprovementsRepo) Resolve(ctx context.Context, id int64, status, resolvedBy string) error {
	if status != "approved" && status != "rejected" {
		return fmt.Errorf("invalid status %q", status)
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE skill_improvements SET status = ?, resolved_by = ?, resolved_at = ? WHERE id = ?
	`, status, resolvedBy, time.Now().Unix(), id)
	return err
}

// CountPending returns the number of pending improvements (for the UI badge).
func (r *SkillImprovementsRepo) CountPending(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM skill_improvements WHERE status = 'pending'`).Scan(&n)
	return n, err
}
