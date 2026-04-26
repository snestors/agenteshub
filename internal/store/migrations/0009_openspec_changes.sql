CREATE TABLE IF NOT EXISTS openspec_changes (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id    INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name          TEXT NOT NULL,
  description   TEXT,
  state         TEXT NOT NULL DEFAULT 'pending_proposal' CHECK(state IN (
    'pending_proposal',
    'awaiting_approval_proposal',
    'awaiting_approval_design',
    'awaiting_approval_tasks',
    'applying',
    'awaiting_approval_verify',
    'archived',
    'rejected'
  )),
  current_phase TEXT NOT NULL DEFAULT 'proposal',
  feedback      TEXT,
  created_at    INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL,
  archived_at   INTEGER,
  UNIQUE(project_id, name)
);

CREATE INDEX IF NOT EXISTS idx_openspec_state ON openspec_changes(state, project_id);
