CREATE TABLE IF NOT EXISTS diagrams (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id      INTEGER REFERENCES projects(id) ON DELETE SET NULL,
  title           TEXT NOT NULL,
  prompt          TEXT,
  mermaid_source  TEXT,
  excalidraw_json TEXT NOT NULL,
  created_at      INTEGER NOT NULL,
  updated_at      INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_diagrams_project ON diagrams(project_id, updated_at DESC);
