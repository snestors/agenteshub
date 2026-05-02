-- Drop the OpenSpec subsystem (f-001).
--
-- 0009 introduced openspec_changes + the SDD/proposal-design-tasks-apply-verify
-- workflow surfaced under /api/projects/{id}/openspec/*. The user decided to
-- replace that workflow with the BettaTech harness (feature_list.json +
-- leader/implementer/reviewer loop), so the entire OpenSpec stack is removed.
--
-- 0015 originally declared conversation_runs.scope with 'openspec' in its
-- CHECK enum. SQLite can't ALTER a CHECK in place, so we rebuild the table
-- without 'openspec' and copy non-openspec rows over. Any conversation_runs
-- rows with scope='openspec' are dropped (they'd be orphaned anyway because
-- their producer code is gone).

PRAGMA foreign_keys = OFF;

BEGIN;

-- 1) Drop the OpenSpec-only table + index from 0009.
DROP INDEX IF EXISTS idx_openspec_state;
DROP TABLE IF EXISTS openspec_changes;

-- 2) Rebuild conversation_runs without 'openspec' in the scope CHECK enum.
CREATE TABLE conversation_runs_new (
  id                       INTEGER PRIMARY KEY AUTOINCREMENT,
  scope                    TEXT NOT NULL CHECK(scope IN ('main','project','agent')),
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

INSERT INTO conversation_runs_new
SELECT id, scope, scope_key, topic, engine, model, session_id, status,
       started_at, updated_at, finished_at, last_error, text_partial,
       thinking_partial, activity_json, result_text, last_seq
FROM conversation_runs
WHERE scope != 'openspec';

DROP TABLE conversation_runs;
ALTER TABLE conversation_runs_new RENAME TO conversation_runs;

CREATE INDEX idx_conversation_runs_scope_status
  ON conversation_runs(scope, status, updated_at DESC);

COMMIT;

PRAGMA foreign_keys = ON;
