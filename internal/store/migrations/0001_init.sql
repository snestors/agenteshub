-- AgentHub Go — initial schema
-- Single SQLite WAL DB. No git for backups; session_snapshots holds JSONL backups.

PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;

-- ============================================================
-- AUTH (reciclado de agenthub-legacy)
-- ============================================================

CREATE TABLE IF NOT EXISTS auth_users (
  id                       INTEGER PRIMARY KEY CHECK (id = 1),
  username                 TEXT NOT NULL UNIQUE,
  password_hash            TEXT NOT NULL,
  totp_secret_encrypted    BLOB NOT NULL,
  created_at               INTEGER NOT NULL,
  last_login               INTEGER
);

CREATE TABLE IF NOT EXISTS jwt_revocations (
  jti                      TEXT PRIMARY KEY,
  expires_at               INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_jwt_revocations_expires_at ON jwt_revocations(expires_at);

-- ============================================================
-- KV — settings simples (engine activo, etc)
-- ============================================================

CREATE TABLE IF NOT EXISTS settings (
  key                      TEXT PRIMARY KEY,
  value                    TEXT NOT NULL,
  updated_at               INTEGER NOT NULL
);

-- ============================================================
-- WHATSAPP — mensajes recibidos / enviados (canal wa + web)
-- ============================================================

CREATE TABLE IF NOT EXISTS wa_messages (
  id                       INTEGER PRIMARY KEY AUTOINCREMENT,
  channel                  TEXT NOT NULL CHECK(channel IN ('wa','web')),
  direction                TEXT NOT NULL CHECK(direction IN ('in','out')),
  jid                      TEXT,
  body                     TEXT,
  media_type               TEXT,                  -- 'image'|'voice'|'audio'|'video'|'document'|NULL
  media_path               TEXT,
  media_caption            TEXT,
  location_lat             REAL,
  location_lng             REAL,
  location_name            TEXT,
  quoted_id                INTEGER REFERENCES wa_messages(id),
  reply_to                 TEXT,                  -- WA message id externo
  ts                       INTEGER NOT NULL,
  is_read                  INTEGER DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_wa_messages_ts ON wa_messages(ts DESC);
CREATE INDEX IF NOT EXISTS idx_wa_messages_channel ON wa_messages(channel);
CREATE INDEX IF NOT EXISTS idx_wa_messages_jid ON wa_messages(jid);
CREATE INDEX IF NOT EXISTS idx_wa_messages_unread ON wa_messages(is_read) WHERE is_read = 0;

CREATE TABLE IF NOT EXISTS wa_jid_map (
  phone                    TEXT PRIMARY KEY,
  jid                      TEXT NOT NULL,
  display_name             TEXT,
  updated_at               INTEGER NOT NULL
);

-- ============================================================
-- TOPICS — espacios de conversación con contexto vivo
-- ============================================================

CREATE TABLE IF NOT EXISTS topics (
  id                       INTEGER PRIMARY KEY AUTOINCREMENT,
  name                     TEXT UNIQUE NOT NULL,        -- 'grid-bot','casa-media','salud','general'
  description              TEXT,
  keywords                 TEXT,                         -- JSON array
  project_id               INTEGER,                      -- FK opcional a projects
  session_id               TEXT NOT NULL,                -- resume id del CLI
  engine                   TEXT NOT NULL DEFAULT 'claude',
  is_default               INTEGER NOT NULL DEFAULT 0,   -- 'general' tiene esto =1
  last_active_at           INTEGER,
  created_at               INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_topics_default ON topics(is_default);

CREATE TABLE IF NOT EXISTS topic_state (
  topic_id                 INTEGER PRIMARY KEY REFERENCES topics(id) ON DELETE CASCADE,
  headline                 TEXT,                         -- una línea con el resumen actual
  active_issues            TEXT,                         -- JSON array
  recent_decisions         TEXT,                         -- JSON array
  pending                  TEXT,                         -- JSON array
  next_action_hint         TEXT,
  last_event_at            INTEGER,
  updated_at               INTEGER NOT NULL
);

-- ============================================================
-- PROJECTS — entornos de coding accesibles desde browser
-- ============================================================

CREATE TABLE IF NOT EXISTS projects (
  id                       INTEGER PRIMARY KEY AUTOINCREMENT,
  name                     TEXT UNIQUE NOT NULL,         -- 'grid-bot','academia','agenthub'
  path                     TEXT NOT NULL,                -- '/home/nestor/grid-bot'
  description              TEXT,
  default_engine           TEXT NOT NULL DEFAULT 'claude',
  created_at               INTEGER NOT NULL,
  updated_at               INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS project_sessions (
  id                       INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id               INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name                     TEXT NOT NULL,                -- 'main','feature-bidget-fix'
  session_id               TEXT NOT NULL,                -- resume id del CLI
  engine                   TEXT NOT NULL,
  summary                  TEXT,                         -- summary liviano
  last_active_at           INTEGER,
  created_at               INTEGER NOT NULL,
  UNIQUE(project_id, engine, name)
);
CREATE INDEX IF NOT EXISTS idx_project_sessions_active ON project_sessions(last_active_at DESC);

-- ============================================================
-- AGENTS — mini-agentes especializados (cron / event / manual)
-- ============================================================

CREATE TABLE IF NOT EXISTS agents (
  id                       INTEGER PRIMARY KEY AUTOINCREMENT,
  name                     TEXT UNIQUE NOT NULL,         -- 'sonarr-watcher','grid-monitor'
  description              TEXT,
  system_prompt            TEXT NOT NULL,
  engine                   TEXT NOT NULL DEFAULT 'claude',
  enabled                  INTEGER NOT NULL DEFAULT 1,
  created_by               TEXT,                         -- 'user'|'main-agent'|nombre
  project_id               INTEGER REFERENCES projects(id) ON DELETE SET NULL,
  created_at               INTEGER NOT NULL,
  updated_at               INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS agent_schedules (
  id                       INTEGER PRIMARY KEY AUTOINCREMENT,
  agent_id                 INTEGER NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
  cron_expr                TEXT NOT NULL,
  prompt_template          TEXT NOT NULL,
  notify_target            TEXT NOT NULL DEFAULT 'main-agent',  -- 'wa:<jid>'|'main-agent'|'topic:<X>'|'none'
  enabled                  INTEGER NOT NULL DEFAULT 1,
  last_run_at              INTEGER,
  next_run                 INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_agent_schedules_next ON agent_schedules(enabled, next_run);

CREATE TABLE IF NOT EXISTS agent_runs (
  id                       INTEGER PRIMARY KEY AUTOINCREMENT,
  agent_id                 INTEGER NOT NULL REFERENCES agents(id),
  schedule_id              INTEGER REFERENCES agent_schedules(id) ON DELETE SET NULL,
  trigger                  TEXT NOT NULL CHECK(trigger IN ('cron','manual','event','main-agent')),
  started_at               INTEGER NOT NULL,
  finished_at              INTEGER,
  status                   TEXT NOT NULL CHECK(status IN ('running','ok','error','cancelled')),
  prompt                   TEXT NOT NULL,
  result                   TEXT,
  tools_used               TEXT,                         -- JSON array
  cost_tokens              INTEGER DEFAULT 0,
  error                    TEXT
);
CREATE INDEX IF NOT EXISTS idx_agent_runs_agent_started ON agent_runs(agent_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_runs_status ON agent_runs(status, started_at DESC);

CREATE TABLE IF NOT EXISTS agent_sessions (
  agent_name               TEXT PRIMARY KEY,             -- 'main-agent', etc.
  engine                   TEXT NOT NULL,
  session_id               TEXT NOT NULL,
  updated_at               INTEGER NOT NULL
);

-- ============================================================
-- AGENT_RECORDS — schema-on-read (datos espontáneos del agente)
-- ============================================================

CREATE TABLE IF NOT EXISTS agent_records (
  id                       INTEGER PRIMARY KEY AUTOINCREMENT,
  topic                    TEXT NOT NULL,                -- 'calories','workouts','books', libre
  data                     TEXT NOT NULL,                -- JSON libre
  agent_name               TEXT,
  source                   TEXT NOT NULL DEFAULT 'agent',
  created_at               INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_agent_records_topic ON agent_records(topic);
CREATE INDEX IF NOT EXISTS idx_agent_records_agent ON agent_records(agent_name);
CREATE INDEX IF NOT EXISTS idx_agent_records_created ON agent_records(created_at DESC);

-- ============================================================
-- SESSION_MESSAGES — capa 3: persistencia humana-legible de turnos
-- ============================================================

CREATE TABLE IF NOT EXISTS session_messages (
  id                       INTEGER PRIMARY KEY AUTOINCREMENT,
  scope                    TEXT NOT NULL CHECK(scope IN ('topic','project','agent','main')),
  topic_id                 INTEGER,
  project_id               INTEGER,
  project_sess_id          INTEGER,
  session_id               TEXT NOT NULL,                -- resume id del CLI
  role                     TEXT NOT NULL CHECK(role IN ('user','assistant','tool','system')),
  body                     TEXT,
  tool_name                TEXT,
  tool_args                TEXT,                         -- JSON
  tool_result              TEXT,
  cost_tokens              INTEGER DEFAULT 0,
  ts                       INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_session_messages_session ON session_messages(session_id, ts);
CREATE INDEX IF NOT EXISTS idx_session_messages_topic ON session_messages(topic_id, ts);
CREATE INDEX IF NOT EXISTS idx_session_messages_project ON session_messages(project_id, project_sess_id, ts);

-- ============================================================
-- SUBAGENT_RUNS — capturados desde stream-json (Agent tool)
-- ============================================================

CREATE TABLE IF NOT EXISTS subagent_runs (
  id                          INTEGER PRIMARY KEY AUTOINCREMENT,
  parent_session_id           TEXT NOT NULL,
  parent_scope                TEXT NOT NULL CHECK(parent_scope IN ('main','topic','project','agent')),
  parent_topic_id             INTEGER,
  parent_project_session_id   INTEGER,
  agent_type                  TEXT,                      -- 'Explore','general-purpose','Plan'
  description                 TEXT,
  prompt                      TEXT,
  result                      TEXT,
  status                      TEXT NOT NULL CHECK(status IN ('running','ok','error','cancelled')),
  started_at                  INTEGER NOT NULL,
  finished_at                 INTEGER,
  cost_tokens                 INTEGER DEFAULT 0,
  tools_used                  TEXT,                      -- JSON array
  worktree_path               TEXT
);
CREATE INDEX IF NOT EXISTS idx_subagent_runs_parent ON subagent_runs(parent_session_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_subagent_runs_status ON subagent_runs(status, started_at DESC);

-- ============================================================
-- SESSION_SNAPSHOTS — capa 2: backup BLOB de las JSONL de Claude
-- ============================================================

CREATE TABLE IF NOT EXISTS session_snapshots (
  session_id               TEXT NOT NULL,
  cwd_dir                  TEXT NOT NULL,                -- '-home-nestor-grid-bot'
  jsonl_data               BLOB NOT NULL,
  jsonl_size               INTEGER,
  snapshot_at              INTEGER NOT NULL,
  PRIMARY KEY (session_id)
);

-- ============================================================
-- FTS5 — full-text search en mensajes y records
-- ============================================================

CREATE VIRTUAL TABLE IF NOT EXISTS wa_messages_fts USING fts5(
  body, jid UNINDEXED, channel UNINDEXED,
  content='wa_messages', content_rowid='id',
  tokenize='porter unicode61 remove_diacritics 1'
);

CREATE TRIGGER IF NOT EXISTS wa_messages_ai AFTER INSERT ON wa_messages BEGIN
  INSERT INTO wa_messages_fts(rowid, body, jid, channel) VALUES (new.id, new.body, new.jid, new.channel);
END;
CREATE TRIGGER IF NOT EXISTS wa_messages_ad AFTER DELETE ON wa_messages BEGIN
  INSERT INTO wa_messages_fts(wa_messages_fts, rowid, body, jid, channel) VALUES ('delete', old.id, old.body, old.jid, old.channel);
END;
CREATE TRIGGER IF NOT EXISTS wa_messages_au AFTER UPDATE ON wa_messages BEGIN
  INSERT INTO wa_messages_fts(wa_messages_fts, rowid, body, jid, channel) VALUES ('delete', old.id, old.body, old.jid, old.channel);
  INSERT INTO wa_messages_fts(rowid, body, jid, channel) VALUES (new.id, new.body, new.jid, new.channel);
END;

CREATE VIRTUAL TABLE IF NOT EXISTS session_messages_fts USING fts5(
  body, session_id UNINDEXED, scope UNINDEXED, role UNINDEXED,
  content='session_messages', content_rowid='id',
  tokenize='porter unicode61 remove_diacritics 1'
);

CREATE TRIGGER IF NOT EXISTS session_messages_ai AFTER INSERT ON session_messages BEGIN
  INSERT INTO session_messages_fts(rowid, body, session_id, scope, role) VALUES (new.id, new.body, new.session_id, new.scope, new.role);
END;

-- ============================================================
-- DEFAULT TOPIC — 'general' como catch-all
-- ============================================================

INSERT OR IGNORE INTO topics(name, description, session_id, engine, is_default, created_at)
VALUES ('general', 'Catch-all sin tema claro', '', 'claude', 1, unixepoch());
