-- Make project session identity engine-aware.
-- A session's resume id + summary belong to exactly one CLI engine, so the UI
-- can keep separate "main" sessions for Claude, Codex, etc. without sharing
-- summaries or accidentally resuming one engine with another engine's id.

PRAGMA foreign_keys = OFF;

CREATE TABLE IF NOT EXISTS project_sessions_v2 (
  id                       INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id               INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name                     TEXT NOT NULL,
  session_id               TEXT NOT NULL,
  engine                   TEXT NOT NULL,
  summary                  TEXT,
  last_active_at           INTEGER,
  created_at               INTEGER NOT NULL,
  UNIQUE(project_id, engine, name)
);

INSERT INTO project_sessions_v2(id, project_id, name, session_id, engine, summary, last_active_at, created_at)
SELECT id, project_id, name, session_id, engine, summary, last_active_at, created_at
FROM project_sessions;

DROP TABLE project_sessions;
ALTER TABLE project_sessions_v2 RENAME TO project_sessions;

CREATE INDEX IF NOT EXISTS idx_project_sessions_active ON project_sessions(last_active_at DESC);
CREATE INDEX IF NOT EXISTS idx_project_sessions_project_engine ON project_sessions(project_id, engine, last_active_at DESC);

PRAGMA foreign_keys = ON;
