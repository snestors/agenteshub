CREATE TABLE IF NOT EXISTS usage_events (
  id                  INTEGER PRIMARY KEY AUTOINCREMENT,
  source              TEXT    NOT NULL,             -- 'claude' | 'codex'
  session_id          TEXT    NOT NULL,
  message_id          TEXT,                         -- dedup Claude (NULL para Codex)
  request_id          TEXT,                         -- dedup Claude (NULL para Codex)
  ts                  INTEGER NOT NULL,             -- unix epoch del evento
  model               TEXT    NOT NULL,
  input_tokens        INTEGER NOT NULL DEFAULT 0,
  output_tokens       INTEGER NOT NULL DEFAULT 0,
  cache_create_tokens INTEGER NOT NULL DEFAULT 0,
  cache_read_tokens   INTEGER NOT NULL DEFAULT 0,
  cost_usd            REAL    NOT NULL DEFAULT 0,
  raw_path            TEXT    NOT NULL,             -- archivo JSONL de origen
  imported_at         INTEGER NOT NULL
);

-- Dedup Claude: un mensaje no puede importarse dos veces.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_usage_claude
  ON usage_events(source, message_id, request_id)
  WHERE source = 'claude';

-- Dedup Codex por (session, ts, input_tokens, output_tokens) — no hay message_id estable.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_usage_codex
  ON usage_events(source, session_id, ts, input_tokens, output_tokens)
  WHERE source = 'codex';

CREATE INDEX IF NOT EXISTS idx_usage_ts    ON usage_events(ts);
CREATE INDEX IF NOT EXISTS idx_usage_model ON usage_events(model);
CREATE INDEX IF NOT EXISTS idx_usage_src   ON usage_events(source, ts);
