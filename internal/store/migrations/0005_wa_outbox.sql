-- 0005_wa_outbox.sql
-- Outbox for outgoing WhatsApp messages produced by the MCP server (which
-- runs as a separate stdio subprocess from the daemon and therefore can't
-- talk to the in-memory wa.Client). The daemon's outbox worker drains this
-- table every ~500ms.

CREATE TABLE IF NOT EXISTS wa_outbox (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  jid         TEXT NOT NULL,
  kind        TEXT NOT NULL CHECK(kind IN ('text','image','voice','audio','document','video','location')),
  body        TEXT,
  media_path  TEXT,
  caption     TEXT,
  loc_lat     REAL,
  loc_lng     REAL,
  loc_name    TEXT,
  status      TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','sent','error')),
  error       TEXT,
  attempts    INTEGER NOT NULL DEFAULT 0,
  created_at  INTEGER NOT NULL,
  sent_at     INTEGER
);

CREATE INDEX IF NOT EXISTS idx_wa_outbox_pending ON wa_outbox(status, created_at)
  WHERE status = 'pending';
