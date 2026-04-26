// Package store wraps the SQLite layer (open, migrate, repos).
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open opens the SQLite database at path, ensuring its parent directory exists,
// applying WAL pragmas, and running embedded migrations.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("ensure db dir: %w", err)
	}
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON", path)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite WAL: one writer at a time
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if err := migrate(ctx, db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS __migrations (
			name TEXT PRIMARY KEY,
			applied_at INTEGER NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("ensure __migrations: %w", err)
	}
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		var c int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(1) FROM __migrations WHERE name = ?`, name).Scan(&c); err != nil {
			return fmt.Errorf("check %s: %w", name, err)
		}
		if c > 0 {
			continue // already applied
		}
		raw, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if _, err := db.ExecContext(ctx, string(raw)); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO __migrations(name, applied_at) VALUES(?, unixepoch())`, name); err != nil {
			return fmt.Errorf("track %s: %w", name, err)
		}
	}
	return nil
}

// Repos bundles all repositories for dependency injection.
type Repos struct {
	Auth      *AuthRepo
	Settings  *SettingsRepo
	Messages  *MessagesRepo
	Topics    *TopicsRepo
	Projects  *ProjectsRepo
	Agents    *AgentsRepo
	Records   *RecordsRepo
	Sessions  *SessionsRepo
	Snapshots *SnapshotsRepo
	Subagents *SubagentRunsRepo
	Secrets   *SecretsRepo
	Diagrams  *DiagramsRepo
	WaOutbox  *WaOutboxRepo
}

// NewRepos wires up every repo around a single *sql.DB handle.
func NewRepos(db *sql.DB) *Repos {
	return &Repos{
		Auth:      NewAuthRepo(db),
		Settings:  NewSettingsRepo(db),
		Messages:  NewMessagesRepo(db),
		Topics:    NewTopicsRepo(db),
		Projects:  NewProjectsRepo(db),
		Agents:    NewAgentsRepo(db),
		Records:   NewRecordsRepo(db),
		Sessions:  NewSessionsRepo(db),
		Snapshots: NewSnapshotsRepo(db),
		Subagents: NewSubagentRunsRepo(db),
		Secrets:   NewSecretsRepo(db),
		Diagrams:  NewDiagramsRepo(db),
		WaOutbox:  NewWaOutboxRepo(db),
	}
}

// rowScanner is implemented by *sql.Row and *sql.Rows.
type rowScanner interface{ Scan(dest ...any) error }
