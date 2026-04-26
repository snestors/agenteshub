package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Snapshot is a BLOB backup of a Claude/Codex JSONL session file.
type Snapshot struct {
	SessionID  string
	CWDDir     string
	JsonlData  []byte
	JsonlSize  int64
	SnapshotAt int64
}

// SnapshotsRepo persists session_snapshots.
type SnapshotsRepo struct{ db *sql.DB }

func NewSnapshotsRepo(db *sql.DB) *SnapshotsRepo { return &SnapshotsRepo{db: db} }

// Upsert overwrites the latest snapshot for a session.
func (r *SnapshotsRepo) Upsert(ctx context.Context, s Snapshot) error {
	if s.SnapshotAt == 0 {
		s.SnapshotAt = time.Now().Unix()
	}
	if s.JsonlSize == 0 {
		s.JsonlSize = int64(len(s.JsonlData))
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO session_snapshots(session_id, cwd_dir, jsonl_data, jsonl_size, snapshot_at)
		VALUES(?,?,?,?,?)
		ON CONFLICT(session_id) DO UPDATE SET
			cwd_dir=excluded.cwd_dir,
			jsonl_data=excluded.jsonl_data,
			jsonl_size=excluded.jsonl_size,
			snapshot_at=excluded.snapshot_at
	`, s.SessionID, s.CWDDir, s.JsonlData, s.JsonlSize, s.SnapshotAt)
	if err != nil {
		return fmt.Errorf("upsert snapshot: %w", err)
	}
	return nil
}

// Get returns the latest snapshot for a session (or nil).
func (r *SnapshotsRepo) Get(ctx context.Context, sessionID string) (*Snapshot, error) {
	var s Snapshot
	err := r.db.QueryRowContext(ctx, `
		SELECT session_id, cwd_dir, jsonl_data, jsonl_size, snapshot_at
		FROM session_snapshots WHERE session_id = ?
	`, sessionID).Scan(&s.SessionID, &s.CWDDir, &s.JsonlData, &s.JsonlSize, &s.SnapshotAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get snapshot: %w", err)
	}
	return &s, nil
}

// All returns all snapshots metadata (without BLOBs).
func (r *SnapshotsRepo) All(ctx context.Context) ([]Snapshot, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT session_id, cwd_dir, jsonl_size, snapshot_at FROM session_snapshots ORDER BY snapshot_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list snapshots: %w", err)
	}
	defer rows.Close()
	out := []Snapshot{}
	for rows.Next() {
		var s Snapshot
		if err := rows.Scan(&s.SessionID, &s.CWDDir, &s.JsonlSize, &s.SnapshotAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
