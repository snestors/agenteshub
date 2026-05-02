-- Persist notifications so the UI can show an unread count + history (0.5.0).
--
-- Until now Notification objects were only broadcast over WS — anyone who
-- wasn't connected when one fired missed it. The new bell + drawer in the
-- topbar wants a queryable history; the simplest store is one row per
-- notification with a nullable read_at.

CREATE TABLE IF NOT EXISTS notifications (
  id            TEXT PRIMARY KEY,           -- broadcastNotification's "n-<base36>" id
  kind          TEXT NOT NULL,              -- 'main_turn_done' | 'main_turn_failed' | 'long_running' | ...
  severity      TEXT NOT NULL DEFAULT 'info', -- 'info' | 'warn' | 'error'
  title         TEXT NOT NULL,
  body          TEXT,
  context_json  TEXT,                       -- JSON blob with kind-specific extras
  created_at    INTEGER NOT NULL,
  read_at       INTEGER                     -- NULL until marked read
);

CREATE INDEX IF NOT EXISTS idx_notif_created  ON notifications(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notif_unread   ON notifications(read_at, created_at DESC);
