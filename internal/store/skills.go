package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Skill is a parsed entry from a SKILL.md file. The DB row is a flat
// projection of the markdown + frontmatter; the canonical source is still
// the file on disk.
type Skill struct {
	ID          int64
	Name        string
	Source      string // 'local' | 'project:<slug>' | 'registry:<name>'
	Description sql.NullString
	RoleHint    sql.NullString
	Version     sql.NullString
	Frontmatter sql.NullString // raw YAML re-serialised as JSON
	Body        string
	Path        string
	PulledAt    int64
	UpdatedAt   int64
}

type SkillsRepo struct{ db *sql.DB }

func NewSkillsRepo(db *sql.DB) *SkillsRepo { return &SkillsRepo{db: db} }

// List returns every skill ordered by name. Optional filter by source.
func (r *SkillsRepo) List(ctx context.Context, source string) ([]Skill, error) {
	q := `SELECT id, name, source, description, role_hint, version, frontmatter, body, path, pulled_at, updated_at
	      FROM skills`
	args := []any{}
	if source != "" {
		q += ` WHERE source = ?`
		args = append(args, source)
	}
	q += ` ORDER BY name ASC`
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	defer rows.Close()
	out := []Skill{}
	for rows.Next() {
		var s Skill
		if err := rows.Scan(&s.ID, &s.Name, &s.Source, &s.Description, &s.RoleHint,
			&s.Version, &s.Frontmatter, &s.Body, &s.Path, &s.PulledAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Upsert inserts or updates by (source, name). Used by the sync routine.
func (r *SkillsRepo) Upsert(ctx context.Context, s Skill) error {
	now := time.Now().Unix()
	if s.PulledAt == 0 {
		s.PulledAt = now
	}
	s.UpdatedAt = now
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO skills(name, source, description, role_hint, version, frontmatter, body, path, pulled_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(source, name) DO UPDATE SET
			description = excluded.description,
			role_hint   = excluded.role_hint,
			version     = excluded.version,
			frontmatter = excluded.frontmatter,
			body        = excluded.body,
			path        = excluded.path,
			pulled_at   = excluded.pulled_at,
			updated_at  = excluded.updated_at
	`, s.Name, s.Source, s.Description, s.RoleHint, s.Version, s.Frontmatter, s.Body, s.Path, s.PulledAt, s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert skill %q@%q: %w", s.Name, s.Source, err)
	}
	return nil
}

// DeleteStale removes rows from a given source whose pulled_at is older than
// `cutoff`. Run after a full sync so files removed upstream disappear here too.
func (r *SkillsRepo) DeleteStale(ctx context.Context, source string, cutoff int64) (int64, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM skills WHERE source = ? AND pulled_at < ?`, source, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
