-- FCM/Web Push device tokens registered by authenticated browsers/PWAs.
CREATE TABLE IF NOT EXISTS push_tokens (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  provider     TEXT NOT NULL DEFAULT 'fcm',
  token        TEXT NOT NULL UNIQUE,
  user_agent   TEXT,
  created_at   INTEGER NOT NULL,
  updated_at   INTEGER NOT NULL,
  last_error   TEXT,
  disabled_at  INTEGER
);

CREATE INDEX IF NOT EXISTS idx_push_tokens_active ON push_tokens(provider, disabled_at);
