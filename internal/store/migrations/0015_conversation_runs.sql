-- Persist live conversation/runtime state so chat activity survives
-- navigation/reloads and project chats can hydrate active/completed runs.

ALTER TABLE session_messages ADD COLUMN activity TEXT;

CREATE TABLE IF NOT EXISTS conversation_runs (
  id                       INTEGER PRIMARY KEY AUTOINCREMENT,
  scope                    TEXT NOT NULL CHECK(scope IN ('main','project','agent','openspec')),
  scope_key                TEXT NOT NULL,
  topic                    TEXT,
  engine                   TEXT NOT NULL,
  model                    TEXT,
  session_id               TEXT,
  status                   TEXT NOT NULL CHECK(status IN ('running','done','error','cancelled','interrupted')),
  started_at               INTEGER NOT NULL,
  updated_at               INTEGER NOT NULL,
  finished_at              INTEGER,
  last_error               TEXT,
  text_partial             TEXT,
  thinking_partial         TEXT,
  activity_json            TEXT,
  result_text              TEXT,
  last_seq                 INTEGER NOT NULL DEFAULT 0,
  UNIQUE(scope, scope_key)
);

CREATE INDEX IF NOT EXISTS idx_conversation_runs_scope_status ON conversation_runs(scope, status, updated_at DESC);
